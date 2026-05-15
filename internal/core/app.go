package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gabiito/zdb/internal/ai"
	"github.com/gabiito/zdb/internal/config"
	"github.com/gabiito/zdb/internal/db"
	"github.com/gabiito/zdb/internal/secrets"
	"github.com/gabiito/zdb/internal/tui"
	"github.com/gabiito/zdb/internal/views"
)

// App is the Bubbletea root model for zDB.
type App struct {
	cfg      config.Config
	snapshot config.Snapshot // load-time fingerprint; updated on every successful save
	log      *slog.Logger
	drv  db.Driver
	ai   ai.AIProvider
	edit *EditBuffer
	exec *Executor

	cache SchemaCache

	screen ScreenID
	modal  Modal
	focus  FocusState

	// Sub-models (initialized as needed)
	connPicker   tui.ConnPickerModel
	schemaBrow   tui.SchemaBrowserModel
	dataViewer   tui.DataViewerModel
	cellEdit     tui.CellEditModel
	sqlBar       tui.SqlBarModel
	askPanel     tui.AskPanelModel
	confirm      tui.ConfirmModel
	stagedView   tui.StagedViewModel
	joinWizard   tui.JoinWizardModel
	joinChoice   tui.JoinChoiceModel
	viewsList    tui.ViewsListModel
	saveView     tui.SaveViewModel
	addConn      tui.AddConnectionModel
	editConn     tui.EditConnectionModel
	welcome      tui.WelcomeModel
	pwPrompt     tui.PasswordPromptModel
	sqlEditor    tui.SQLEditorModel
	aiSetup      tui.AISetupModel
	aiDebug      tui.AIDebugModel
	aiProfiles   tui.AIProfileListModel
	aiAnalytics  tui.AIAnalyticsModel
	pendingDeleteAIProfile string // name pending delete confirm
	statusBar    tui.StatusBarModel
	spinner      spinner.Model

	// Global shortcuts overlay (Ctrl+,). When showShortcuts is true the
	// overlay swallows input and renders full-screen on top of whatever
	// view was active.
	shortcuts     tui.ShortcutsModel
	showShortcuts bool

	viewsStore *views.Store
	configPath string // resolved at startup, used to persist new connections
	lastSQL    string

	// pendingStatusMsg is a one-shot status-bar message emitted from Init().
	// Set before the TUI starts (e.g. from legacy migration in main.go) so
	// the message appears on the first rendered frame.
	pendingStatusMsg string

	// Pending password / DSN template captured between AddConnectionSubmit
	// and the testConnResult callback. The password lives only in memory
	// for the duration of the test.
	pendingPassword    string
	pendingTemplateDSN string

	// Edit-flow state captured between EditConnectionSubmit and the test
	// callback. pendingEditOriginal.Name == "" means no edit is in flight.
	pendingEditOriginal        config.Connection
	pendingEditPassword        string
	pendingEditPasswordChanged bool

	// Connection name pending delete-confirmation. Empty when no delete is
	// in flight. When non-empty, ConfirmYes/No routes to the delete handler
	// instead of the transaction commit/rollback path.
	pendingDeleteConn string

	// Connection captured between ConnectMsg (when a password prompt is
	// required) and the prompt's submit/cancel. Empty Name means no prompt
	// is in flight.
	pendingConnectConn config.Connection

	// AI flow tracking — used to route DB errors that originated from an
	// AI-generated query into the debug panel (where the user can hint
	// the AI to fix it) instead of just dropping them in the status bar.
	lastAIQuestion string
	lastAIQuerySQL string
	aiQueryActive  bool

	// Database-level pagination state for the active table view.
	// dbOffset is the offset of the FIRST row of the current buffer;
	// pageDir is the direction of the in-flight page fetch (0 idle,
	// +1 replace forward, -1 replace backward, +2 append forward — the
	// infinite-scroll path triggered by ↓/j at the last buffer row).
	// dbNextOffset is the offset that page-replace requests will commit
	// once the result returns successfully (we cannot derive it post-hoc
	// because the buffer length changes when the append path is used).
	dbOffset     int
	dbPageSize   int
	pageDir      int
	dbNextOffset int

	// Tab strip state. tabs[0] is always the fixed Schema tab; tabs[1:]
	// are data tabs (table view or SQL result). activeTab indexes the
	// currently-focused tab. lastDataTab tracks the most recently active
	// data tab so opening a table from the schema tab can reuse it
	// without forcing a new tab on every Enter.
	tabs        []*Tab
	activeTab   int
	lastDataTab int

	// joinChain mirrors the JOIN sequence currently materialized in the data
	// viewer (alias + table per step). Empty when no wizard-built JOIN is
	// active. Reset when a regular table is opened.
	joinChain []joinChainStep

	// Current active connection name (for messages)
	connName string

	// In-flight request tracking: reqID → cancel function
	inflight map[string]context.CancelFunc

	// Open transaction (during confirm flow)
	pendingTx db.Tx

	// Terminal dimensions
	width, height int
}

// NewApp creates a new App from a loaded config.
// The LoadedConfig carries both the parsed Config and the load-time snapshot;
// the snapshot is threaded through saves to detect external modifications.
func NewApp(loaded config.LoadedConfig, log *slog.Logger) *App {
	cfg := loaded.Config
	s := spinner.New()
	s.Spinner = spinner.Dot

	provider := ai.New(resolveAIConfig(cfg.ActiveProfile()))

	// viewsStore is nil until the first successful ConnectedMsg — the store
	// is re-initialised per-connection in the ConnectedMsg handler (Slice 4).
	configPath, err := config.ResolvePath()
	if err != nil {
		log.Warn("config path resolution failed", "err", err)
	}

	if !secrets.Available() {
		log.Warn("OS keyring not reachable — new connections that need a password will fail to persist; use dsn_env in config.toml as a fallback")
	}
	for _, c := range cfg.Connections {
		if secrets.LooksLikePlaintextPassword(c.Engine, c.DSN) {
			log.Warn("plaintext password detected in config", "connection", c.Name, "advice", "re-add this connection via 'n' so the password moves to the OS keyring, or replace dsn with dsn_env")
		}
	}

	startScreen := ScreenConnPicker
	if len(cfg.Connections) == 0 {
		startScreen = ScreenWelcome
	}

	return &App{
		cfg:         cfg,
		snapshot:    loaded.Snapshot,
		log:         log,
		ai:          provider,
		edit:        &EditBuffer{},
		exec:        &Executor{},
		screen:      startScreen,
		inflight:    make(map[string]context.CancelFunc),
		spinner:     s,
		sqlBar:      tui.NewSqlBarModel(80),
		configPath:  configPath,
		lastDataTab: -1,
	}
}

// SetPendingStatusMsg stores a one-shot status-bar message that is emitted as
// a Cmd from Init(). Call this before handing the App to tea.NewProgram.
func (a *App) SetPendingStatusMsg(msg string) {
	a.pendingStatusMsg = msg
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	cmds := []tea.Cmd{
		a.spinner.Tick,
		// Status bar will be initialized on first WindowSizeMsg
	}
	if a.pendingStatusMsg != "" {
		msg := a.pendingStatusMsg
		a.pendingStatusMsg = ""
		cmds = append(cmds, func() tea.Msg { return tui.StatusSetMsg{Text: msg} })
	}
	return tea.Batch(cmds...)
}

// Update implements tea.Model — the central event dispatcher.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Global: always handle window size
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		a.width = wsMsg.Width
		a.height = wsMsg.Height
		a.statusBar.SetWidth(wsMsg.Width)
		a.shortcuts.SetSize(a.width, a.height)

		// Re-initialize the active screen's sub-model when its layout depends
		// on the terminal size.
		switch a.screen {
		case ScreenConnPicker:
			a.connPicker = tui.NewConnPickerModel(a.cfg.Connections, a.width, a.height)
		case ScreenWelcome:
			a.welcome = tui.NewWelcomeModel(a.width, a.height)
		case ScreenSQLEditor:
			a.sqlEditor.SetSize(a.width, a.height)
		}
		return a, nil
	}

	// Global: Ctrl+C to quit
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.Type == tea.KeyCtrlC {
			// Cancel any in-flight requests
			for _, cancel := range a.inflight {
				cancel()
			}
			return a, tea.Quit
		}
	}

	// Global: F1 toggles the shortcuts overlay. When active, the overlay
	// receives all key input until it dismisses itself.
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "f1" {
			a.showShortcuts = !a.showShortcuts
			if a.showShortcuts {
				a.shortcuts = tui.NewShortcutsModel(a.width, a.height)
			}
			return a, nil
		}
		if a.showShortcuts {
			var close bool
			a.shortcuts, close = a.shortcuts.Update(keyMsg)
			if close {
				a.showShortcuts = false
			}
			return a, nil
		}
	}

	// Route messages
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case tui.ClearStatusMsg:
		cmds = append(cmds, a.statusBar.Update(msg))

	case tui.StatusSetMsg:
		if msg.IsErr {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("%s", msg.Text)))
		} else {
			cmds = append(cmds, a.statusBar.SetMsg(msg.Text))
		}

	case tui.ConnectMsg:
		// Ask-at-connect: when the DSN carries a `{password}` placeholder
		// without any backing secret (no keyring key, no env var), pop a
		// modal asking the user for the password. The connect command
		// runs only after the prompt resolves.
		if msg.Conn.KeyringKey == "" && msg.Conn.DSNEnv == "" &&
			strings.Contains(msg.Conn.DSN, secrets.PasswordPlaceholder) {
			a.pendingConnectConn = msg.Conn
			a.pwPrompt = tui.NewPasswordPromptModel(msg.Conn.Name, a.width, a.height)
			a.modal = ModalPasswordPrompt
			break
		}
		cmds = append(cmds, a.connectCmd(msg.Conn))

	case tui.PasswordPromptSubmitMsg:
		if a.pendingConnectConn.Name == "" {
			a.modal = ModalNone
			break
		}
		conn := a.pendingConnectConn
		full, err := secrets.InjectPassword(conn.Engine, conn.DSN, msg.Password)
		if err != nil {
			a.pwPrompt.SetError(err.Error())
			break
		}
		conn.DSN = full
		// Clear keyring/env hints so connectCmd → ResolveDSN returns the DSN
		// as-is (it already has the password substituted).
		conn.KeyringKey = ""
		conn.DSNEnv = ""
		a.pendingConnectConn = config.Connection{}
		a.modal = ModalNone
		cmds = append(cmds, a.connectCmd(conn))

	case tui.PasswordPromptCancelMsg:
		a.pendingConnectConn = config.Connection{}
		a.modal = ModalNone

	case tui.AISetupSubmitMsg:
		// Find or create the profile.
		idx := -1
		for i := range a.cfg.AIs {
			if a.cfg.AIs[i].Name == msg.Name {
				idx = i
				break
			}
		}
		// Per-profile keyring key keeps multiple profiles' secrets isolated.
		keyringKey := ""
		if idx >= 0 {
			keyringKey = a.cfg.AIs[idx].KeyringKey
		}
		if msg.APIKey != "" {
			if keyringKey == "" {
				keyringKey = keyringKeyForProfile(msg.Name)
			}
			if err := secrets.SetPassword(keyringKey, msg.APIKey); err != nil {
				a.aiSetup.SetError(fmt.Sprintf("OS keyring unavailable: %v", err))
				break
			}
		}
		profile := config.AIProfile{
			Name:           msg.Name,
			Provider:       "openai-compat",
			BaseURL:        msg.BaseURL,
			Model:          msg.Model,
			TimeoutSeconds: msg.TimeoutSeconds,
			KeyringKey:     keyringKey,
		}
		if idx >= 0 {
			a.cfg.AIs[idx] = profile
		} else {
			a.cfg.AIs = append(a.cfg.AIs, profile)
		}
		// New profiles activate automatically; edits keep the current
		// active selection unless this WAS the active one (no-op).
		if !msg.IsEdit || a.cfg.ActiveAI == "" {
			a.cfg.ActiveAI = msg.Name
		}
		if a.configPath == "" {
			a.aiSetup.SetError("no config path resolved — cannot persist AI settings")
			break
		}
		// Modal closes on success; backup-skip annotation would not be
		// visible to the user — using Save() wrapper intentionally.
		// (REQ-6.4 carve-out: zero Snapshot skips external-mod check; design §5.3)
		if err := config.Save(a.cfg, a.configPath); err != nil {
			a.aiSetup.SetError(fmt.Sprintf("save config: %v", err))
			break
		}
		// Re-initialize the provider so subsequent Asks use the new config.
		a.ai = ai.New(resolveAIConfig(a.cfg.ActiveProfile()))
		a.modal = ModalNone
		if msg.IsEdit {
			cmds = append(cmds, a.statusBar.SetMsg("profile updated: "+msg.Name))
		} else {
			cmds = append(cmds, a.statusBar.SetMsg("AI configured — opening Ask Panel"))
			a.screen = ScreenAskPanel
			a.askPanel = tui.NewAskPanelModel(true, a.width, a.height)
		}

	case tui.AISetupCancelMsg:
		a.modal = ModalNone

	case ConnectedMsg:
		if _, live := a.inflight[msg.ReqID]; !live {
			break // cancelled
		}
		delete(a.inflight, msg.ReqID)
		if msg.Err != nil {
			a.log.Error("connection failed", "name", msg.ConnName, "err", msg.Err)
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[conn] %s: connection failed", msg.ConnName)))
		} else {
			a.connName = msg.ConnName
			cmds = append(cmds, a.statusBar.SetMsg("Connected: "+msg.ConnName))
			// Introspect schema
			cmds = append(cmds, a.introspectCmd())
		}

	case SchemaBuiltMsg:
		if _, live := a.inflight[msg.ReqID]; !live {
			break
		}
		delete(a.inflight, msg.ReqID)
		if msg.Err != nil {
			a.log.Error("schema introspection failed", "err", msg.Err)
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[db] schema introspection failed")))
		} else {
			// Schema is already stored in cache by the cmd; transition to schema browser
			a.schemaBrow = tui.NewSchemaBrowserModel(a.cache.Tables(), &a.cache, a.width, a.height)
			a.refreshSqlBarSchema()
			a.initTabs() // sets activeTab = 0 (Schema) and clears lastDataTab
			a.screen = ScreenSchemaBrowser
		}

	case tui.OpenTableMsg:
		if msg.Table != nil {
			// Tab routing: by default reuse the active data tab (or the
			// most recently active one if we're currently on the schema
			// tab). Ctrl+Enter explicitly forces a new tab.
			a.saveActiveDataTab()

			targetIdx := -1
			if !msg.NewTab {
				// Reuse the active data tab if any.
				if a.activeTab > 0 && a.tabs[a.activeTab].Kind == TabData {
					targetIdx = a.activeTab
				} else if a.lastDataTab > 0 {
					targetIdx = a.lastDataTab
				}
			}
			if targetIdx < 0 {
				targetIdx = a.addDataTab(msg.Table.Name)
			} else {
				a.tabs[targetIdx].Title = msg.Table.Name
				a.activeTab = targetIdx
				a.lastDataTab = targetIdx
			}

			a.screen = ScreenDataViewer
			a.dbPageSize = 50
			a.dbOffset = 0
			a.pageDir = 0
			a.dataViewer = tui.NewDataViewerModel(msg.Table, a.dbPageSize, a.width, a.height)
			a.dataViewer.SetDBOffset(0)
			a.joinChain = nil // opening a table abandons any prior JOIN chain
			// Kick off both the first page load AND a count query in parallel.
			// The count is a one-shot per table-open and feeds the
			// "Loaded N/T" display in the status line.
			cmds = append(cmds, a.queryCmd(msg.Table, 0, a.dbPageSize))
			cmds = append(cmds, a.countCmd(msg.Table))
		}

	case TableCountMsg:
		if _, live := a.inflight[msg.ReqID]; !live {
			break
		}
		delete(a.inflight, msg.ReqID)
		if msg.Err != nil {
			// Count failure isn't fatal — just leave the total unknown
			// (the data viewer hides the suffix when totalRows < 0).
			a.log.Warn("count failed", "table", msg.Table, "err", msg.Err)
			break
		}
		// Only apply if the user is still on the same table — by the time
		// the count returns they may have navigated elsewhere.
		if tbl := a.dataViewer.Table(); tbl != nil && tbl.Name == msg.Table {
			a.dataViewer.SetTotalRows(msg.Total)
		}

	case tui.WantNextPageMsg:
		tbl := a.dataViewer.Table()
		if tbl == nil {
			cmds = append(cmds, a.statusBar.SetMsg("derived results — paging is only supported on tables"))
			break
		}
		a.pageDir = 1
		// "Next page" starts where the current buffer ENDS — important
		// because the buffer may have grown beyond dbPageSize via append.
		a.dbNextOffset = a.dbOffset + a.dataViewer.LoadedRowCount()
		cmds = append(cmds, a.queryCmd(tbl, a.dbNextOffset, a.dbPageSize))

	case tui.WantPrevPageMsg:
		if a.dbOffset == 0 {
			cmds = append(cmds, a.statusBar.SetMsg("already at the first page"))
			break
		}
		tbl := a.dataViewer.Table()
		if tbl == nil {
			break
		}
		a.pageDir = -1
		a.dbNextOffset = a.dbOffset - a.dbPageSize
		if a.dbNextOffset < 0 {
			a.dbNextOffset = 0
		}
		cmds = append(cmds, a.queryCmd(tbl, a.dbNextOffset, a.dbPageSize))

	case tui.WantNextPageAppendMsg:
		tbl := a.dataViewer.Table()
		if tbl == nil {
			cmds = append(cmds, a.statusBar.SetMsg("derived results — paging is only supported on tables"))
			break
		}
		a.pageDir = 2
		appendOffset := a.dbOffset + a.dataViewer.LoadedRowCount()
		cmds = append(cmds, a.queryCmd(tbl, appendOffset, a.dbPageSize))

	case DBQueryDoneMsg:
		if _, live := a.inflight[msg.ReqID]; !live {
			break
		}
		delete(a.inflight, msg.ReqID)
		if msg.Err != nil {
			// AI-driven query failed — surface it in the debug panel so
			// the user can hint the AI for a fix instead of just losing
			// the failure context to the status bar.
			if a.aiQueryActive {
				a.aiQueryActive = false
				a.aiDebug = tui.NewAIDebugModel(
					a.lastAIQuestion,
					a.lastAIQuerySQL,
					msg.Err.Error(),
					a.width,
					a.height,
				)
				a.modal = ModalAIDebug
				a.pageDir = 0
				break
			}
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[db] query: %v", msg.Err)))
			a.pageDir = 0
			break
		}
		// Successful query — clear AI tracking so the next failure (if
		// it comes from a non-AI path) doesn't trigger the debug panel.
		a.aiQueryActive = false
		isEmpty := msg.ResultSet == nil || len(msg.ResultSet.Rows) == 0
		// Forward fetches (page-replace or append) on an empty result mean
		// we ran past the end of the table — leave the buffer intact and
		// tell the user.
		if (a.pageDir == 1 || a.pageDir == 2) && isEmpty {
			cmds = append(cmds, a.statusBar.SetMsg("no more rows"))
			a.pageDir = 0
			break
		}
		switch a.pageDir {
		case 1:
			a.dbOffset = a.dbNextOffset
			a.dataViewer.SetData(msg.ResultSet)
			a.dataViewer.SetDBOffset(a.dbOffset)
			a.dataViewer.MoveToTop()
		case -1:
			a.dbOffset = a.dbNextOffset
			a.dataViewer.SetData(msg.ResultSet)
			a.dataViewer.SetDBOffset(a.dbOffset)
			a.dataViewer.MoveToBottom()
		case 2:
			// Append: extend the buffer and put the cursor on the first
			// newly-loaded row so the user keeps moving forward naturally.
			prevLen := a.dataViewer.LoadedRowCount()
			a.dataViewer.AppendRows(msg.ResultSet)
			a.dataViewer.SetCursorRow(prevLen)
		default:
			// Initial table load or raw SQL result.
			a.dataViewer.SetData(msg.ResultSet)
		}
		a.pageDir = 0

	case DBExecDoneMsg:
		if _, live := a.inflight[msg.ReqID]; !live {
			break
		}
		delete(a.inflight, msg.ReqID)
		if msg.Err != nil {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[db] exec: %v", msg.Err)))
		}

	case ErrMsg:
		a.log.Error("async error", "source", msg.Source, "err", msg.Err)
		cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[%s] %v", msg.Source, msg.Err)))

	case ConnLostMsg:
		a.log.Warn("connection lost", "name", msg.ConnName)
		cmds = append(cmds, a.statusBar.SetMsg("Reconnecting: "+msg.ConnName))
		cmds = append(cmds, a.reconnectCmd())

	case tui.ConfirmYesMsg:
		if a.pendingDeleteConn != "" {
			cmds = append(cmds, a.deleteConnection(a.pendingDeleteConn))
			a.pendingDeleteConn = ""
			a.modal = ModalNone
			break
		}
		if a.pendingDeleteAIProfile != "" {
			cmds = append(cmds, a.deleteAIProfile(a.pendingDeleteAIProfile))
			a.pendingDeleteAIProfile = ""
			a.openAIProfileList() // re-open the list with the entry gone
			break
		}
		if a.pendingTx != nil {
			cmds = append(cmds, a.commitCmd(a.pendingTx))
			a.pendingTx = nil
		}
		a.modal = ModalNone

	case tui.ConfirmNoMsg:
		if a.pendingDeleteConn != "" {
			a.pendingDeleteConn = ""
			a.modal = ModalNone
			break
		}
		if a.pendingDeleteAIProfile != "" {
			a.pendingDeleteAIProfile = ""
			a.openAIProfileList()
			break
		}
		if a.pendingTx != nil {
			cmds = append(cmds, a.rollbackCmd(a.pendingTx))
			a.pendingTx = nil
		}
		a.modal = ModalNone

	case tui.StagedChangeMsg:
		if err := a.edit.Stage(
			msg.Table, msg.PK, msg.Col, msg.OldVal, msg.NewVal,
		); err != nil {
			cmds = append(cmds, a.statusBar.SetErr(err))
		}
		a.modal = ModalNone

	case tui.DiscardEditMsg:
		a.modal = ModalNone

	case applyReadyMsg:
		// Edit apply succeeded; Tx is open — show diff for confirmation
		a.pendingTx = msg.tx
		a.confirm = tui.NewConfirmModel("Commit changes?\n"+msg.diff, false)
		a.modal = ModalConfirm

	case deleteReadyMsg:
		// Delete tx is open — store for confirmation
		a.pendingTx = msg.tx

	case txCommittedMsg:
		if msg.Err != nil {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[db] commit: %v", msg.Err)))
		} else {
			a.edit.Clear()
			cmds = append(cmds, a.statusBar.SetMsg("changes saved"))
			if cmd := a.refreshCurrentTable(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case txRolledBackMsg:
		if msg.Err != nil {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[db] rollback: %v", msg.Err)))
		} else {
			a.edit.Clear()
			cmds = append(cmds, a.statusBar.SetMsg("changes discarded"))
		}

	case tui.SQLEditorRunMsg:
		sql := strings.TrimSpace(msg.SQL)
		if sql == "" {
			cmds = append(cmds, a.statusBar.SetMsg("nothing to run"))
			break
		}
		a.lastSQL = sql
		a.screen = ScreenDataViewer
		cmds = append(cmds, a.execSQLCmd(sql))

	case tui.SQLEditorSaveViewMsg:
		sql := strings.TrimSpace(msg.SQL)
		if sql == "" {
			cmds = append(cmds, a.statusBar.SetMsg("nothing to save"))
			break
		}
		a.lastSQL = sql
		a.saveView = tui.NewSaveViewModel(sql, a.width, a.height)
		a.modal = ModalSaveView

	case tui.SQLEditorCancelMsg:
		// Going back from the editor — preserve buffer for next open.
		// If a table was previously open, return to the data viewer;
		// otherwise to the schema browser; otherwise to the picker.
		if a.dataViewer.Table() != nil {
			a.screen = ScreenDataViewer
		} else if a.cache.Get() != nil {
			a.screen = ScreenSchemaBrowser
		} else {
			a.screen = ScreenConnPicker
		}

	case tui.SqlExecuteMsg:
		// Filter-on-JOIN: when the user is viewing a JOIN result and types
		// only a WHERE / ORDER BY / GROUP BY / HAVING / LIMIT clause, treat
		// the input as a filter to append to the active JOIN SQL instead
		// of running it as a fresh query. A full SELECT (or any other DML)
		// still replaces. The wizard always emits clean SELECT JOINs with
		// no trailing clauses, so direct concatenation is safe.
		sql := msg.SQL
		joinFiltered := false
		if len(a.joinChain) >= 2 && a.lastSQL != "" && isFilterClause(sql) {
			sql = strings.TrimSpace(a.lastSQL) + " " + strings.TrimSpace(sql)
			joinFiltered = true
		}
		a.lastSQL = sql
		cmds = append(cmds, a.execSQLCmd(sql))
		a.screen = ScreenDataViewer
		if joinFiltered {
			cmds = append(cmds, a.statusBar.SetMsg("filter applied to JOIN result"))
		}

	case tui.JoinExecMsg:
		a.modal = ModalNone
		a.lastSQL = msg.SQL
		if msg.AppendedTable != "" {
			// Extend mode: append to the existing chain.
			a.joinChain = append(a.joinChain, joinChainStep{tableName: msg.AppendedTable, alias: msg.AppendedAlias})
		} else if tbl := a.dataViewer.Table(); tbl != nil {
			// Fresh JOIN from a base table — reset chain to [base, picked-right].
			// We don't know the picked-right name from the message in fresh mode,
			// but the wizard built SQL `FROM left a JOIN right b`, so the chain
			// after the first wizard run is parsed from the chain bookkeeping
			// the wizard didn't expose. Pragmatic approach: reset to just the
			// base table and let the next extension start fresh aliases. (b)
			a.joinChain = []joinChainStep{{tableName: tbl.Name, alias: "a"}}
			// The chain still needs the right table; we infer from msg.SQL by
			// extracting the first JOIN target.
			if rt := firstJoinedTable(msg.SQL); rt != "" {
				a.joinChain = append(a.joinChain, joinChainStep{tableName: rt, alias: "b"})
			}
		}
		cmds = append(cmds, a.execSQLCmd(msg.SQL))
		cmds = append(cmds, a.statusBar.SetMsg("running JOIN…"))

	case tui.JoinCancelMsg:
		a.modal = ModalNone

	case tui.JoinChoiceMsg:
		switch {
		case msg.Add:
			// Extend: open wizard pre-filled with last chain table as LEFT.
			last := a.joinChain[len(a.joinChain)-1]
			leftTbl := a.cache.Table(last.tableName)
			if leftTbl == nil {
				cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[join] last chain table %q missing from schema", last.tableName)))
				a.modal = ModalNone
			} else {
				a.joinWizard = tui.NewJoinWizardModelExtend(
					leftTbl, &a.cache, a.lastSQL, last.alias, a.nextJoinAlias(),
					a.width, a.height,
				)
				a.modal = ModalJoinWizard
			}
		case msg.Replace:
			// Replace: clear chain, open wizard fresh on the data viewer's
			// current table. If the viewer has no table (derived results),
			// fall back to canceling.
			a.joinChain = nil
			if tbl := a.dataViewer.Table(); tbl != nil {
				a.joinWizard = tui.NewJoinWizardModel(tbl, &a.cache, a.width, a.height)
				a.modal = ModalJoinWizard
			} else {
				a.modal = ModalNone
				cmds = append(cmds, a.statusBar.SetMsg("no base table — open one first to start a fresh JOIN"))
			}
		default:
			a.modal = ModalNone
		}

	case tui.RunViewMsg:
		a.modal = ModalNone
		a.lastSQL = msg.SQL
		a.screen = ScreenDataViewer
		cmds = append(cmds, a.execSQLCmd(msg.SQL))

	case tui.DeleteViewMsg:
		if a.viewsStore != nil {
			if err := a.viewsStore.Remove(msg.Name); err != nil {
				cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[views] delete: %v", err)))
			} else {
				cmds = append(cmds, a.statusBar.SetMsg("view deleted: "+msg.Name))
				a.viewsList = tui.NewViewsListModel(a.loadViewItems(), a.width, a.height)
			}
		}

	case tui.CloseViewsMsg:
		a.modal = ModalNone

	case tui.SaveViewSubmitMsg:
		a.modal = ModalNone
		if a.viewsStore == nil {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[views] store unavailable")))
		} else if err := a.viewsStore.Add(views.View{Name: msg.Name, SQL: msg.SQL}); err != nil {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[views] save: %v", err)))
		} else {
			cmds = append(cmds, a.statusBar.SetMsg("view saved: "+msg.Name))
		}

	case tui.SaveViewCancelMsg:
		a.modal = ModalNone

	case tui.AddConnectionSubmitMsg:
		conn := msg.Connection

		if a.cfg.HasConnectionNamed(conn.Name) {
			a.addConn.SetError("a connection with that name already exists")
			break
		}

		// Ask-at-connect intent: empty password field on a non-sqlite engine,
		// either with a `{password}` placeholder typed explicitly or with no
		// password info at all in the DSN. Save without a keyring entry, skip
		// the test — the password will be requested at connect time.
		if msg.Password == "" && conn.Engine != "sqlite" {
			toSave := conn
			needsAsk := false
			if strings.Contains(conn.DSN, secrets.PasswordPlaceholder) {
				needsAsk = true
			} else {
				_, _, hasEmbedded := secrets.SplitDSN(conn.Engine, conn.DSN)
				if !hasEmbedded {
					template, err := secrets.InjectPlaceholder(conn.Engine, conn.DSN)
					if err != nil {
						a.addConn.SetError(err.Error())
						break
					}
					toSave.DSN = template
					needsAsk = true
				}
			}
			if needsAsk {
				a.cfg.Connections = append(a.cfg.Connections, toSave)
				if a.configPath == "" {
					cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[config] no path resolved — connection added in-memory only")))
				} else {
					cmds = append(cmds, a.saveConfigAnnotated("connection saved (asks password on connect): "+toSave.Name))
				}
				a.modal = ModalNone
				if a.screen == ScreenWelcome {
					a.screen = ScreenConnPicker
				}
				if a.screen == ScreenConnPicker {
					a.connPicker = tui.NewConnPickerModel(a.cfg.Connections, a.width, a.height)
				}
				break
			}
		}

		// Standard path: a password was supplied (form field or embedded in
		// DSN). Test the connection, then persist via keyring on success.
		if msg.Password != "" {
			full, err := secrets.InjectPassword(conn.Engine, conn.DSN, msg.Password)
			if err != nil {
				a.addConn.SetError(err.Error())
				break
			}
			conn.DSN = full
		}
		a.pendingPassword = msg.Password
		a.pendingTemplateDSN = msg.Connection.DSN
		a.addConn.SetTesting(true)
		cmds = append(cmds, a.testConnectionCmd(conn))

	case testConnResultMsg:
		a.addConn.SetTesting(false)
		if msg.err != nil {
			a.addConn.SetTestError(msg.err.Error())
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[conn-test] %s: %v", msg.conn.Name, msg.err)))
			a.pendingPassword = ""
			a.pendingTemplateDSN = ""
			break
		}
		// Test passed — split out any password into the keyring, then persist.
		toSave, err := a.persistConnection(msg.conn)
		if err != nil {
			a.addConn.SetError(err.Error())
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[secrets] %v", err)))
			a.pendingPassword = ""
			a.pendingTemplateDSN = ""
			break
		}
		a.pendingPassword = ""
		a.pendingTemplateDSN = ""
		a.cfg.Connections = append(a.cfg.Connections, toSave)
		if a.configPath == "" {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[config] no path resolved — connection added in-memory only")))
		} else {
			suffix := ""
			if toSave.KeyringKey != "" {
				suffix = " (password stored in OS keyring)"
			}
			cmds = append(cmds, a.saveConfigAnnotated("connection saved: "+toSave.Name+suffix))
		}
		a.modal = ModalNone
		// First-run flow: if we were on the welcome screen, advance to the
		// connection picker now that there's something to pick.
		if a.screen == ScreenWelcome {
			a.screen = ScreenConnPicker
		}
		if a.screen == ScreenConnPicker {
			a.connPicker = tui.NewConnPickerModel(a.cfg.Connections, a.width, a.height)
		}

	case tui.AddConnectionCancelMsg:
		a.pendingPassword = ""
		a.pendingTemplateDSN = ""
		a.modal = ModalNone

	case tui.WelcomeAddConnectionMsg:
		a.addConn = tui.NewAddConnectionModel(a.width, a.height)
		a.modal = ModalAddConnection

	case tui.WelcomeQuitMsg:
		return a, tea.Quit

	case tui.EditConnectionSubmitMsg:
		conn := msg.Updated

		if !strings.EqualFold(conn.Name, msg.Original.Name) && a.cfg.HasConnectionNamed(conn.Name) {
			a.editConn.SetError("a connection with that name already exists")
			break
		}

		// Original was ask-at-connect (placeholder DSN, no keyring, no env)
		// and the user didn't supply a new password — there's nothing to
		// test with, so save directly and stay in ask-at-connect mode.
		if !msg.PasswordChanged &&
			msg.Original.KeyringKey == "" &&
			msg.Original.DSNEnv == "" &&
			strings.Contains(msg.Original.DSN, secrets.PasswordPlaceholder) {
			for i, c := range a.cfg.Connections {
				if c.Name == msg.Original.Name {
					a.cfg.Connections[i] = msg.Updated
					break
				}
			}
			if a.configPath == "" {
				cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[config] no path resolved — connection updated in-memory only")))
			} else {
				cmds = append(cmds, a.saveConfigAnnotated("connection updated: "+msg.Updated.Name))
			}
			a.modal = ModalNone
			if a.screen == ScreenConnPicker {
				a.connPicker = tui.NewConnPickerModel(a.cfg.Connections, a.width, a.height)
			}
			break
		}

		if msg.PasswordChanged && msg.Password != "" {
			full, err := secrets.InjectPassword(conn.Engine, conn.DSN, msg.Password)
			if err != nil {
				a.editConn.SetError(err.Error())
				break
			}
			conn.DSN = full
		} else if msg.Original.KeyringKey != "" || msg.Original.DSNEnv != "" {
			// No new password supplied — resolve the existing secret to test
			// the (possibly changed) DSN. The saved DSN remains a template.
			resolved, err := secrets.ResolveDSN(conn.Engine, conn.DSN, msg.Original.KeyringKey, msg.Original.DSNEnv)
			if err != nil {
				a.editConn.SetError(err.Error())
				break
			}
			conn.DSN = resolved
		}
		a.pendingEditOriginal = msg.Original
		a.pendingEditPassword = msg.Password
		a.pendingEditPasswordChanged = msg.PasswordChanged
		a.editConn.SetTesting(true)
		cmds = append(cmds, a.testEditConnectionCmd(conn))

	case tui.EditConnectionCancelMsg:
		a.pendingEditOriginal = config.Connection{}
		a.pendingEditPassword = ""
		a.pendingEditPasswordChanged = false
		a.modal = ModalNone

	case editTestConnResultMsg:
		a.editConn.SetTesting(false)
		if msg.err != nil {
			a.editConn.SetTestError(msg.err.Error())
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[conn-test] %s: %v", msg.conn.Name, msg.err)))
			a.pendingEditOriginal = config.Connection{}
			a.pendingEditPassword = ""
			a.pendingEditPasswordChanged = false
			break
		}
		toSave, err := a.persistEditedConnection(a.pendingEditOriginal, msg.conn, a.pendingEditPassword, a.pendingEditPasswordChanged)
		if err != nil {
			a.editConn.SetError(err.Error())
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[secrets] %v", err)))
			a.pendingEditOriginal = config.Connection{}
			a.pendingEditPassword = ""
			a.pendingEditPasswordChanged = false
			break
		}
		// Replace the original entry in cfg.Connections by name.
		for i, c := range a.cfg.Connections {
			if c.Name == a.pendingEditOriginal.Name {
				a.cfg.Connections[i] = toSave
				break
			}
		}
		a.pendingEditOriginal = config.Connection{}
		a.pendingEditPassword = ""
		a.pendingEditPasswordChanged = false
		if a.configPath == "" {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[config] no path resolved — connection updated in-memory only")))
		} else {
			cmds = append(cmds, a.saveConfigAnnotated("connection updated: "+toSave.Name))
		}
		a.modal = ModalNone
		if a.screen == ScreenConnPicker {
			a.connPicker = tui.NewConnPickerModel(a.cfg.Connections, a.width, a.height)
		}

	case tui.AskSubmitMsg:
		a.lastAIQuestion = msg.Question
		a.askPanel.SetLoading(true)
		cmds = append(cmds, a.askAICmd(msg.Question))

	case AISuggestDoneMsg:
		if _, live := a.inflight[msg.ReqID]; !live {
			break
		}
		delete(a.inflight, msg.ReqID)
		if msg.Err != nil {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[ai] suggest: %v", msg.Err)))
		}
		// Truncation warning
		if msg.Truncated {
			cmds = append(cmds, a.statusBar.SetMsg("AI: schema truncated to 30 tables"))
		}

	case AIAskDoneMsg:
		if _, live := a.inflight[msg.ReqID]; !live {
			break
		}
		delete(a.inflight, msg.ReqID)
		a.askPanel.SetLoading(false)
		if msg.Err != nil {
			// Surface AI failures in the debug panel when one is open
			// (a retry came back empty/failed) so the user keeps the
			// failure context. Otherwise just status-bar it.
			if a.modal == ModalAIDebug {
				a.aiDebug.SetPending(false)
				cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[ai] ask: %v", msg.Err)))
			} else {
				cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[ai] ask: %v", msg.Err)))
			}
			break
		}
		// Successful AI response. If we're recovering from a failure, the
		// debug panel was open; close it before running the new SQL.
		if a.modal == ModalAIDebug {
			a.modal = ModalNone
		}
		// Auto-execute read-only queries and route the result through the
		// data viewer; preview-and-confirm any mutating SQL via the ask
		// panel so the AI never writes without explicit user OK.
		if isReadOnlySQL(msg.SQL) {
			a.lastAIQuerySQL = msg.SQL
			a.aiQueryActive = true
			a.lastSQL = msg.SQL
			a.screen = ScreenDataViewer
			cmds = append(cmds, a.execSQLCmd(msg.SQL))
		} else {
			a.askPanel.SetPreview(msg.SQL)
		}
		if msg.Truncated {
			cmds = append(cmds, a.statusBar.SetMsg("AI: schema truncated to 30 tables"))
		}

	case tui.AIDebugRetryMsg:
		// Build a retry prompt that gives the AI the failure context
		// alongside the user's hint, then re-issue the Ask. The debug
		// panel stays visible (in pending state) until the response.
		retry := buildAIDebugPrompt(msg.Question, msg.PreviousSQL, msg.Error, msg.Hint)
		a.aiDebug.SetPending(true)
		cmds = append(cmds, a.askAICmd(retry))

	case tui.AIDebugCancelMsg:
		a.modal = ModalNone
		a.aiQueryActive = false

	case tui.AIDebugEditMsg:
		a.modal = ModalNone
		a.aiQueryActive = false
		a.openSQLEditor()
		a.sqlEditor.SetValue(msg.SQL)

	case tui.AIProfileActivateMsg:
		a.cfg.ActiveAI = msg.Name
		// Wire via helper: gates the success message on a successful write
		// (fixes latent bug where success was appended unconditionally —
		// REQ-28). configPath == "" is safe: helper emits a SetErr cmd.
		cmds = append(cmds, a.saveConfigAnnotated("AI profile: "+msg.Name))
		a.ai = ai.New(resolveAIConfig(a.cfg.ActiveProfile()))
		a.aiProfiles = tui.NewAIProfileListModel(a.cfg.AIs, a.cfg.ActiveAI, a.width, a.height)

	case tui.AIProfileAddMsg:
		a.aiSetup = tui.NewAISetupModel(a.width, a.height)
		a.modal = ModalAISetup

	case tui.AIProfileEditMsg:
		for _, p := range a.cfg.AIs {
			if p.Name == msg.Name {
				a.aiSetup = tui.NewAISetupModelEdit(p.Name, p.BaseURL, p.Model, a.width, a.height)
				a.modal = ModalAISetup
				break
			}
		}

	case tui.AIProfileDeleteMsg:
		a.pendingDeleteAIProfile = msg.Name
		a.confirm = tui.NewConfirmModel(
			fmt.Sprintf("Delete AI profile %q?", msg.Name),
			true,
		)
		a.modal = ModalConfirm

	case tui.AIProfileListCloseMsg:
		a.modal = ModalNone

	case tui.AIProfileOpenAnalyticsMsg:
		records, err := ai.LoadUsage()
		a.aiAnalytics = tui.NewAIAnalyticsModel(records, err, a.width, a.height)
		a.modal = ModalAIAnalytics

	case tui.AIAnalyticsCloseMsg:
		a.modal = ModalNone

	default:
		// Route to active screen / modal
		var cmd tea.Cmd
		cmds = append(cmds, a.routeToScreen(msg))
		_ = cmd
	}

	return a, tea.Batch(cmds...)
}

// routeToScreen dispatches a message to the active screen's sub-model.
func (a *App) routeToScreen(msg tea.Msg) tea.Cmd {
	var cmds []tea.Cmd

	// Modal takes priority over everything else.
	if a.modal != ModalNone {
		switch a.modal {
		case ModalCellEdit:
			var cmd tea.Cmd
			a.cellEdit, cmd = a.cellEdit.Update(msg)
			cmds = append(cmds, cmd)
		case ModalConfirm:
			var cmd tea.Cmd
			a.confirm, cmd = a.confirm.Update(msg)
			cmds = append(cmds, cmd)
		case ModalCellView:
			var cmd tea.Cmd
			a.dataViewer, cmd = a.dataViewer.UpdateCellView(msg)
			cmds = append(cmds, cmd)
		case ModalJoinWizard:
			var cmd tea.Cmd
			a.joinWizard, cmd = a.joinWizard.Update(msg)
			cmds = append(cmds, cmd)
		case ModalJoinChoice:
			var cmd tea.Cmd
			a.joinChoice, cmd = a.joinChoice.Update(msg)
			cmds = append(cmds, cmd)
		case ModalViewsList:
			var cmd tea.Cmd
			a.viewsList, cmd = a.viewsList.Update(msg)
			cmds = append(cmds, cmd)
		case ModalSaveView:
			var cmd tea.Cmd
			a.saveView, cmd = a.saveView.Update(msg)
			cmds = append(cmds, cmd)
		case ModalAddConnection:
			var cmd tea.Cmd
			a.addConn, cmd = a.addConn.Update(msg)
			cmds = append(cmds, cmd)
		case ModalEditConnection:
			var cmd tea.Cmd
			a.editConn, cmd = a.editConn.Update(msg)
			cmds = append(cmds, cmd)
		case ModalPasswordPrompt:
			var cmd tea.Cmd
			a.pwPrompt, cmd = a.pwPrompt.Update(msg)
			cmds = append(cmds, cmd)
		case ModalAISetup:
			var cmd tea.Cmd
			a.aiSetup, cmd = a.aiSetup.Update(msg)
			cmds = append(cmds, cmd)
		case ModalAIDebug:
			var cmd tea.Cmd
			a.aiDebug, cmd = a.aiDebug.Update(msg)
			cmds = append(cmds, cmd)
		case ModalAIProfileList:
			var cmd tea.Cmd
			a.aiProfiles, cmd = a.aiProfiles.Update(msg)
			cmds = append(cmds, cmd)
		case ModalAIAnalytics:
			var cmd tea.Cmd
			a.aiAnalytics, cmd = a.aiAnalytics.Update(msg)
			cmds = append(cmds, cmd)
		case ModalStagedView:
			handled := false
			if keyMsg, ok := msg.(tea.KeyMsg); ok {
				switch keyMsg.String() {
				case "esc", "q":
					a.modal = ModalNone
					handled = true
				case "s":
					a.modal = ModalNone
					if len(a.edit.Changes()) > 0 {
						cmds = append(cmds, a.applyEditsCmd())
					}
					handled = true
				case "D":
					a.modal = ModalNone
					cmds = append(cmds, a.discardStagedCmd())
					handled = true
				}
			}
			if !handled {
				var cmd tea.Cmd
				a.stagedView, cmd = a.stagedView.Update(msg)
				cmds = append(cmds, cmd)
			}
		}
		return tea.Batch(cmds...)
	}

	// SQL bar focus takes priority over screen routing once no modal is open.
	if a.sqlBar.IsFocused() {
		isTab := false
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "esc":
				a.sqlBar.Blur()
				return tea.Batch(cmds...)
			case "enter":
				sql := strings.TrimSpace(a.sqlBar.Value())
				a.sqlBar.Blur()
				if sql != "" {
					a.sqlBar.Clear()
					a.screen = ScreenDataViewer
					cmds = append(cmds, a.execSQLCmd(sql))
				}
				return tea.Batch(cmds...)
			case "tab":
				isTab = true
			}
		}
		var cmd tea.Cmd
		a.sqlBar, cmd = a.sqlBar.Update(msg)
		cmds = append(cmds, cmd)
		if isTab {
			if cur, idx, total := a.sqlBar.CurrentCompletion(); cur != "" {
				cmds = append(cmds, a.statusBar.SetMsg(fmt.Sprintf("completion %d/%d: %s — Tab cycles", idx, total, cur)))
			} else {
				cmds = append(cmds, a.statusBar.SetMsg("no completions for the current word"))
			}
		}
		return tea.Batch(cmds...)
	}

	// Active screen
	switch a.screen {
	case ScreenWelcome:
		var cmd tea.Cmd
		a.welcome, cmd = a.welcome.Update(msg)
		cmds = append(cmds, cmd)
	case ScreenSQLEditor:
		var cmd tea.Cmd
		a.sqlEditor, cmd = a.sqlEditor.Update(msg)
		cmds = append(cmds, cmd)
	case ScreenConnPicker:
		var cmd tea.Cmd
		a.connPicker, cmd = a.connPicker.Update(msg)
		cmds = append(cmds, cmd)
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "n":
				a.addConn = tui.NewAddConnectionModel(a.width, a.height)
				a.modal = ModalAddConnection
			case "e":
				if sel, ok := a.connPicker.Selected(); ok {
					a.editConn = tui.NewEditConnectionModel(sel, a.width, a.height)
					a.modal = ModalEditConnection
				}
			case "d":
				if sel, ok := a.connPicker.Selected(); ok {
					a.pendingDeleteConn = sel.Name
					a.confirm = tui.NewConfirmModel(
						fmt.Sprintf("Delete connection %q?", sel.Name),
						true,
					)
					a.modal = ModalConfirm
				}
			}
		}
	case ScreenSchemaBrowser:
		var cmd tea.Cmd
		a.schemaBrow, cmd = a.schemaBrow.Update(msg)
		cmds = append(cmds, cmd)
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "esc":
				a.screen = ScreenConnPicker
			case "s":
				if len(a.edit.Changes()) > 0 {
					cmds = append(cmds, a.applyEditsCmd())
				}
			case "S":
				a.stagedView = tui.NewStagedViewModel(a.edit.Diff(), len(a.edit.Changes()), a.width, a.height)
				a.modal = ModalStagedView
			case "D":
				cmds = append(cmds, a.discardStagedCmd())
			case ":":
				a.refreshSqlBarPlaceholder()
				cmds = append(cmds, a.sqlBar.Focus())
			case "ctrl+e":
				a.openSQLEditor()
			case "ctrl+a", "f2":
				a.openAI()
			case "ctrl+p":
				a.openAIProfileList()
			case "V":
				a.viewsList = tui.NewViewsListModel(a.loadViewItems(), a.width, a.height)
				a.modal = ModalViewsList
			case "ctrl+right":
				if len(a.tabs) > 1 {
					a.activateTab((a.activeTab + 1) % len(a.tabs))
				}
			case "ctrl+left":
				if len(a.tabs) > 1 {
					a.activateTab((a.activeTab - 1 + len(a.tabs)) % len(a.tabs))
				}
			}
		}
	case ScreenDataViewer:
		var cmd tea.Cmd
		a.dataViewer, cmd = a.dataViewer.Update(msg)
		cmds = append(cmds, cmd)
		// Handle key actions from data viewer
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "s": // save staged changes
				if len(a.edit.Changes()) > 0 {
					cmds = append(cmds, a.applyEditsCmd())
				}
			case "S": // show staged-changes modal
				a.stagedView = tui.NewStagedViewModel(a.edit.Diff(), len(a.edit.Changes()), a.width, a.height)
				a.modal = ModalStagedView
			case "J": // open the join-wizard modal (or choice if a chain is active)
				if len(a.joinChain) >= 2 {
					a.joinChoice = tui.NewJoinChoiceModel(a.joinChainTables(), a.width, a.height)
					a.modal = ModalJoinChoice
				} else if tbl := a.dataViewer.Table(); tbl != nil {
					a.joinWizard = tui.NewJoinWizardModel(tbl, &a.cache, a.width, a.height)
					a.modal = ModalJoinWizard
				}
			case "V": // open the saved-views list
				a.viewsList = tui.NewViewsListModel(a.loadViewItems(), a.width, a.height)
				a.modal = ModalViewsList
			case "W": // save the last-run SQL as a named view
				if a.lastSQL == "" {
					cmds = append(cmds, a.statusBar.SetMsg("nothing to save — run SQL or a join first"))
				} else {
					a.saveView = tui.NewSaveViewModel(a.lastSQL, a.width, a.height)
					a.modal = ModalSaveView
				}
			case "D": // discard all staged changes
				cmds = append(cmds, a.discardStagedCmd())
			case ":": // focus the inline SQL bar
				a.refreshSqlBarPlaceholder()
				cmds = append(cmds, a.sqlBar.Focus())
			case "ctrl+e": // open the full-screen SQL editor
				a.openSQLEditor()
			case "ctrl+a", "f2": // open AI ask panel (or setup wizard when unconfigured)
				a.openAI()
			case "v": // view cell
				if row, col, ok := a.dataViewer.SelectedCell(); ok {
					a.dataViewer.OpenCellView(row, col)
					a.modal = ModalCellView
				}
			case "enter": // edit cell under cursor
				if row, col, ok := a.dataViewer.SelectedCell(); ok {
					table := a.dataViewer.Table()
					if table == nil {
						cmds = append(cmds, a.statusBar.SetMsg("derived results — open a table to edit"))
					} else if len(table.PKCols) == 0 {
						cmds = append(cmds, a.statusBar.SetMsg("table has no primary key — read-only"))
					} else {
						colName := a.dataViewer.ColumnName(col)
						var colDef db.Column
						found := false
						for _, c := range table.Columns {
							if c.Name == colName {
								colDef = c
								found = true
								break
							}
						}
						if !found {
							cmds = append(cmds, a.statusBar.SetMsg("column not in schema — cannot edit"))
						} else if colDef.IsPK {
							cmds = append(cmds, a.statusBar.SetMsg("cannot edit primary key column"))
						} else {
							pk := a.dataViewer.RowPK(row)
							a.cellEdit = tui.NewCellEditModel(row, col, a.dataViewer.CellValue(row, col)).
								WithTableContext(table, colDef, pk)
							a.modal = ModalCellEdit
						}
					}
				}
			case "y": // copy current cell value to clipboard
				if row, col, ok := a.dataViewer.SelectedCell(); ok {
					val := a.dataViewer.CellValue(row, col)
					if err := clipboard.WriteAll(val); err != nil {
						cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[clipboard] %v", err)))
					} else {
						cmds = append(cmds, a.statusBar.SetMsg("cell copied"))
					}
				}
			case "Y": // copy marked rows (or current row) as TSV with header
				tsv, n := a.copyRowsTSV()
				if n == 0 {
					cmds = append(cmds, a.statusBar.SetMsg("nothing to copy"))
				} else if err := clipboard.WriteAll(tsv); err != nil {
					cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[clipboard] %v", err)))
				} else {
					word := "row"
					if n != 1 {
						word = "rows"
					}
					cmds = append(cmds, a.statusBar.SetMsg(fmt.Sprintf("%d %s copied (TSV)", n, word)))
				}
			case " ": // toggle mark on current row, sets range anchor
				a.dataViewer.ToggleMark()
			case "shift+space", "M": // mark range from anchor to current row
				a.dataViewer.MarkRange()
			case "d": // delete row
				if row, ok := a.dataViewer.SelectedRow(); ok {
					table := a.dataViewer.Table()
					if table == nil {
						cmds = append(cmds, a.statusBar.SetMsg("derived results — open a table to delete"))
					} else if len(table.PKCols) == 0 {
						cmds = append(cmds, a.statusBar.SetMsg("table has no primary key — read-only"))
					} else {
						pk := a.dataViewer.RowPK(row)
						a.confirm = tui.NewConfirmModel(
							fmt.Sprintf("Delete row WHERE %v?", pk),
							true, // red style
						)
						a.modal = ModalConfirm
						// Store delete context for when user confirms
						cmds = append(cmds, a.prepareDeleteCmd(table, pk))
					}
				}
			case "esc":
				// First Esc clears any active row marks so the user doesn't
				// jump screens by accident with a selection live. Second Esc
				// (or Esc with no marks) goes back to the Schema tab.
				if a.dataViewer.HasMarks() {
					a.dataViewer.ClearMarks()
				} else {
					for id, cancel := range a.inflight {
						cancel()
						delete(a.inflight, id)
					}
					a.saveActiveDataTab()
					a.activateTab(0) // Schema tab
				}
			case "ctrl+w": // close active data tab
				idx := a.activeTab
				if idx > 0 {
					a.closeTab(idx)
				}
			case "ctrl+right":
				a.saveActiveDataTab()
				if len(a.tabs) > 1 {
					a.activateTab((a.activeTab + 1) % len(a.tabs))
				}
			case "ctrl+left":
				a.saveActiveDataTab()
				if len(a.tabs) > 1 {
					a.activateTab((a.activeTab - 1 + len(a.tabs)) % len(a.tabs))
				}
			}
		}
	case ScreenAskPanel:
		var cmd tea.Cmd
		a.askPanel, cmd = a.askPanel.Update(msg)
		cmds = append(cmds, cmd)
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			if !a.askPanel.IsActive() {
				a.screen = ScreenDataViewer
			}
		}
	}

	return tea.Batch(cmds...)
}

// View implements tea.Model.
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Loading..."
	}

	// Shortcuts overlay short-circuits everything else.
	if a.showShortcuts {
		return a.shortcuts.View()
	}

	// Chrome at the bottom (always pinned): help line + status line.
	spinnerLine := ""
	if len(a.inflight) > 0 {
		spinnerLine = a.spinner.View() + " working…"
	}
	statusLine := a.statusBar.View()
	if spinnerLine != "" {
		statusLine = lipgloss.JoinHorizontal(lipgloss.Left,
			a.statusBar.View(),
			" ",
			spinnerLine,
		)
	}
	helpLine := tui.RenderHelpBar(a.helpContext(), a.width, a.helpState())

	chromeBeneath := lipgloss.Height(helpLine) + lipgloss.Height(statusLine)
	bodyHeight := a.height - chromeBeneath
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	// Tell screens that respect a height budget how much they have. The data
	// viewer uses this to compute how many rows fit; the staged banner above
	// and the SQL bar below each eat lines when present.
	stagedBannerLines := 0
	if len(a.edit.Changes()) > 0 {
		stagedBannerLines = 1
	}
	a.sqlBar.SetWidth(a.width)
	sqlBarLines := 0
	if a.screen == ScreenDataViewer || a.screen == ScreenSchemaBrowser {
		sqlBarLines = a.sqlBar.Height()
	}
	tabBarLine := a.renderTabBar()
	tabBarLines := 0
	if tabBarLine != "" && (a.screen == ScreenDataViewer || a.screen == ScreenSchemaBrowser) {
		tabBarLines = 1
	}
	innerHeight := bodyHeight - stagedBannerLines - sqlBarLines - tabBarLines
	a.dataViewer.SetWidth(a.width)
	a.dataViewer.SetHeight(innerHeight)
	a.applyJoinLegend()
	if a.screen == ScreenSchemaBrowser {
		a.schemaBrow.SetSize(a.width, innerHeight)
	}

	var body string
	switch a.screen {
	case ScreenWelcome:
		body = lipgloss.Place(a.width, bodyHeight, lipgloss.Center, lipgloss.Center, a.welcome.View())
	case ScreenSQLEditor:
		body = a.sqlEditor.View()
	case ScreenConnPicker:
		body = a.connPicker.View()
	case ScreenSchemaBrowser:
		body = a.schemaBrow.View() + "\n" + a.sqlBar.View()
		if tabBarLines > 0 {
			body = tabBarLine + "\n" + body
		}
		body = a.prependStagedBanner(body)
	case ScreenDataViewer:
		body = a.dataViewer.View() + "\n" + a.sqlBar.View()
		if tabBarLines > 0 {
			body = tabBarLine + "\n" + body
		}
		body = a.prependStagedBanner(body)
	case ScreenAskPanel:
		body = a.askPanel.View()
	}

	// Modal overlay — centered within the body area, replacing whatever was
	// rendered underneath. This guarantees the modal is fully visible and
	// the help/status lines below stay pinned.
	if a.modal != ModalNone {
		var overlay string
		switch a.modal {
		case ModalCellEdit:
			overlay = a.cellEdit.View()
		case ModalConfirm:
			overlay = a.confirm.View()
		case ModalCellView:
			overlay = a.dataViewer.CellViewView()
		case ModalStagedView:
			overlay = a.stagedView.View()
		case ModalJoinWizard:
			overlay = a.joinWizard.View()
		case ModalJoinChoice:
			overlay = a.joinChoice.View()
		case ModalViewsList:
			overlay = a.viewsList.View()
		case ModalSaveView:
			overlay = a.saveView.View()
		case ModalAddConnection:
			overlay = a.addConn.View()
		case ModalEditConnection:
			overlay = a.editConn.View()
		case ModalPasswordPrompt:
			overlay = a.pwPrompt.View()
		case ModalAISetup:
			overlay = a.aiSetup.View()
		case ModalAIDebug:
			overlay = a.aiDebug.View()
		case ModalAIProfileList:
			overlay = a.aiProfiles.View()
		case ModalAIAnalytics:
			overlay = a.aiAnalytics.View()
		}
		body = lipgloss.Place(a.width, bodyHeight, lipgloss.Center, lipgloss.Center, overlay)
	}

	// Constrain the body so help/status are pinned to the bottom of the
	// terminal regardless of how tall the body would naturally render.
	body = lipgloss.NewStyle().
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		Render(body)

	return lipgloss.JoinVertical(lipgloss.Left, body, helpLine, statusLine)
}

// discardStagedCmd clears the edit buffer immediately and returns a status
// message. Triggered by the 'D' shortcut from any screen or modal.
func (a *App) discardStagedCmd() tea.Cmd {
	n := len(a.edit.Changes())
	if n == 0 {
		return a.statusBar.SetMsg("no staged changes to discard")
	}
	a.edit.Clear()
	word := "change"
	if n != 1 {
		word = "changes"
	}
	return a.statusBar.SetMsg(fmt.Sprintf("discarded %d %s", n, word))
}

// prependStagedBanner adds the yellow "N changes staged" banner above the body
// when the edit buffer has pending changes. Used by both the data viewer and
// the schema browser so the staged status is visible across navigation.
func (a *App) prependStagedBanner(body string) string {
	n := len(a.edit.Changes())
	if n == 0 {
		return body
	}
	word := "change"
	if n != 1 {
		word = "changes"
	}
	banner := tui.StyleBannerYellow.Render(
		fmt.Sprintf("✎ %d %s staged — 's' save · 'S' review", n, word),
	)
	return banner + "\n" + body
}

// helpState assembles the dynamic context the help bar consumes — what's
// staged, marked, whether the cursor sits at the buffer boundary with more
// rows behind it, etc. The helpbar package uses this to hide irrelevant
// keys and fold counts into descriptions, which keeps the bar narrow
// enough to survive truncation on small terminals.
func (a *App) helpState() tui.HelpState {
	state := tui.HelpState{
		AIEnabled:   a.ai.Enabled(),
		StagedCount: len(a.edit.Changes()),
		MarkCount:   a.dataViewer.MarkCount(),
	}
	if row, col, ok := a.dataViewer.SelectedCell(); ok {
		state.HasResultSet = true
		state.SelectedCol = col
		loaded := a.dataViewer.LoadedRowCount()
		if loaded > 0 {
			state.AtLastLoadedRow = (row == loaded-1)
		}
		if total := a.dataViewer.TotalRows(); total > 0 {
			state.MoreRowsAvailable = (a.dbOffset + loaded) < total
		}
	}
	state.HasJoinChain = len(a.joinChain) >= 2
	state.TabCount = len(a.tabs)
	return state
}

// helpContext maps the current screen/modal state to a tui.HelpContext.
// The modal takes priority over the SQL bar, which takes priority over the
// active screen.
func (a *App) helpContext() tui.HelpContext {
	switch a.modal {
	case ModalCellEdit:
		return tui.HelpContextModalCellEdit
	case ModalConfirm:
		return tui.HelpContextModalConfirm
	case ModalCellView:
		return tui.HelpContextModalCellView
	case ModalStagedView:
		return tui.HelpContextModalStagedView
	case ModalJoinWizard:
		return tui.HelpContextModalJoinWizard
	case ModalJoinChoice:
		return tui.HelpContextModalJoinChoice
	case ModalViewsList:
		return tui.HelpContextModalViewsList
	case ModalSaveView:
		return tui.HelpContextModalSaveView
	case ModalAddConnection:
		return tui.HelpContextModalAddConnection
	case ModalEditConnection:
		return tui.HelpContextModalEditConnection
	case ModalPasswordPrompt:
		return tui.HelpContextModalPasswordPrompt
	case ModalAISetup:
		return tui.HelpContextModalAISetup
	case ModalAIDebug:
		return tui.HelpContextModalAIDebug
	case ModalAIProfileList:
		return tui.HelpContextModalAIProfileList
	case ModalAIAnalytics:
		return tui.HelpContextModalAIAnalytics
	}
	if a.sqlBar.IsFocused() {
		return tui.HelpContextSqlBarFocused
	}
	switch a.screen {
	case ScreenSchemaBrowser:
		return tui.HelpContextSchemaBrowser
	case ScreenDataViewer:
		return tui.HelpContextDataViewer
	case ScreenAskPanel:
		return tui.HelpContextAskPanel
	case ScreenWelcome:
		return tui.HelpContextWelcome
	case ScreenSQLEditor:
		return tui.HelpContextSQLEditor
	}
	return tui.HelpContextConnPicker
}

// ---- Async command builders ----

// connectCmd connects to the given connection profile. The DSN is resolved
// via secrets.ResolveDSN, which substitutes the password from the OS
// keyring or reads the entire DSN from an env var when configured.
func (a *App) connectCmd(conn config.Connection) tea.Cmd {
	reqID := newReqID()
	ctx, cancel := context.WithCancel(context.Background())
	a.inflight[reqID] = cancel

	resolvedDSN, err := secrets.ResolveDSN(conn.Engine, conn.DSN, conn.KeyringKey, conn.DSNEnv)
	if err != nil {
		cancel()
		delete(a.inflight, reqID)
		return a.statusBar.SetErr(fmt.Errorf("[config] %s: %v", conn.Name, err))
	}

	drv, err := db.New(conn.Engine)
	if err != nil {
		cancel()
		delete(a.inflight, reqID)
		return a.statusBar.SetErr(fmt.Errorf("[config] %s: %v", conn.Name, err))
	}
	a.drv = drv

	return func() tea.Msg {
		err := drv.Connect(ctx, resolvedDSN)
		return ConnectedMsg{ReqID: reqID, ConnName: conn.Name, Err: err}
	}
}

// introspectCmd builds the schema cache from the active driver.
func (a *App) introspectCmd() tea.Cmd {
	reqID := newReqID()
	ctx, cancel := context.WithCancel(context.Background())
	a.inflight[reqID] = cancel

	drv := a.drv
	cache := &a.cache

	return func() tea.Msg {
		err := cache.Build(ctx, drv)
		schema := cache.Get()
		return SchemaBuiltMsg{ReqID: reqID, Schema: schema, Err: err}
	}
}

// queryCmd queries a table and returns the result.
func (a *App) queryCmd(table *db.Table, offset, limit int) tea.Cmd {
	reqID := newReqID()
	ctx, cancel := context.WithCancel(context.Background())
	a.inflight[reqID] = cancel

	drv := a.drv
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d", table.Name, limit, offset)

	return func() tea.Msg {
		rs, err := drv.Query(ctx, query)
		if err != nil {
			return DBQueryDoneMsg{ReqID: reqID, Err: err}
		}
		return DBQueryDoneMsg{ReqID: reqID, ResultSet: rs}
	}
}

// countCmd issues a COUNT(*) for the given table and returns the result
// in a TableCountMsg. Fired once per OpenTableMsg so the data viewer can
// display "Loaded N / total T" in its status line. A failure here is
// non-fatal — paging/data still work without the total.
func (a *App) countCmd(table *db.Table) tea.Cmd {
	reqID := newReqID()
	ctx, cancel := context.WithCancel(context.Background())
	a.inflight[reqID] = cancel

	drv := a.drv
	tableName := table.Name
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)

	return func() tea.Msg {
		rs, err := drv.Query(ctx, query)
		if err != nil {
			return TableCountMsg{ReqID: reqID, Table: tableName, Err: err}
		}
		if rs == nil || len(rs.Rows) == 0 || len(rs.Rows[0].Cells) == 0 {
			return TableCountMsg{ReqID: reqID, Table: tableName, Total: 0}
		}
		// Drivers return COUNT(*) as int64 (or compatible). Defensive
		// type-switch handles every shape we've seen across the three
		// engines; anything else falls back to 0.
		total := 0
		switch v := rs.Rows[0].Cells[0].(type) {
		case int64:
			total = int(v)
		case int32:
			total = int(v)
		case int:
			total = v
		case float64:
			total = int(v)
		case []byte:
			// MySQL's go-sql-driver may return COUNT(*) as []byte
			// depending on driver version / DSN flags.
			if n, perr := strconv.Atoi(string(v)); perr == nil {
				total = n
			}
		case string:
			if n, perr := strconv.Atoi(v); perr == nil {
				total = n
			}
		}
		return TableCountMsg{ReqID: reqID, Table: tableName, Total: total}
	}
}

// applyEditsCmd applies staged changes via the executor.
func (a *App) applyEditsCmd() tea.Cmd {
	if a.drv == nil || len(a.edit.Changes()) == 0 {
		return nil
	}
	drv := a.drv
	edit := a.edit
	exec := a.exec
	diff := edit.Diff()

	return func() tea.Msg {
		tx, err := exec.Apply(context.Background(), drv, edit)
		if err != nil {
			return ErrMsg{Source: "db", Err: err}
		}
		// Store tx in App via a special message
		return applyReadyMsg{tx: tx, diff: diff}
	}
}

// applyReadyMsg is sent when Apply succeeds and a Tx is open for confirmation.
type applyReadyMsg struct {
	tx   db.Tx
	diff string
}

// prepareDeleteCmd opens a delete transaction and stores it for confirmation.
func (a *App) prepareDeleteCmd(table *db.Table, pk map[string]any) tea.Cmd {
	drv := a.drv
	exec := a.exec

	return func() tea.Msg {
		tx, err := exec.Delete(context.Background(), drv, table, pk)
		if err != nil {
			return ErrMsg{Source: "db", Err: err}
		}
		return deleteReadyMsg{tx: tx}
	}
}

// deleteReadyMsg is sent when a DELETE tx is open and awaiting confirmation.
type deleteReadyMsg struct {
	tx db.Tx
}

// txCommittedMsg is emitted after Commit returns. The Update handler is
// responsible for clearing the edit buffer and refreshing the data viewer.
type txCommittedMsg struct{ Err error }

// txRolledBackMsg is emitted after Rollback returns.
type txRolledBackMsg struct{ Err error }

// commitCmd commits a pending transaction.
func (a *App) commitCmd(tx db.Tx) tea.Cmd {
	return func() tea.Msg {
		return txCommittedMsg{Err: tx.Commit(context.Background())}
	}
}

// rollbackCmd rolls back a pending transaction.
func (a *App) rollbackCmd(tx db.Tx) tea.Cmd {
	return func() tea.Msg {
		return txRolledBackMsg{Err: tx.Rollback(context.Background())}
	}
}

// joinChainStep records one entry in the active JOIN chain.
type joinChainStep struct {
	tableName string
	alias     string
}

// applyJoinLegend pushes the joinChain summary into the data viewer as a
// legend line, plus per-column alias prefixes when the column count matches
// the chain's tables (the unambiguous SELECT * case).
func (a *App) applyJoinLegend() {
	if len(a.joinChain) < 2 {
		a.dataViewer.SetLegend("")
		a.dataViewer.SetColumnPrefixes(nil)
		return
	}

	parts := make([]string, len(a.joinChain))
	expected := 0
	for i, step := range a.joinChain {
		parts[i] = step.alias + "=" + step.tableName
		if t := a.cache.Table(step.tableName); t != nil {
			expected += len(t.Columns)
		}
	}
	a.dataViewer.SetLegend(strings.Join(parts, " · "))

	if expected == a.dataViewer.ResultSetColumnCount() {
		prefixes := make([]string, 0, expected)
		for _, step := range a.joinChain {
			t := a.cache.Table(step.tableName)
			if t == nil {
				continue
			}
			for range t.Columns {
				prefixes = append(prefixes, step.alias)
			}
		}
		a.dataViewer.SetColumnPrefixes(prefixes)
	} else {
		// Specific cols path — column-to-alias mapping isn't unambiguous.
		a.dataViewer.SetColumnPrefixes(nil)
	}
}

// firstJoinedTable extracts the table name immediately following the first
// `JOIN` keyword in a SQL string. Returns "" when no JOIN is found. Naive —
// expects the SQL to have come from the wizard (no quoted identifiers etc.).
func firstJoinedTable(sql string) string {
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, " JOIN ")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(sql[idx+len(" JOIN "):])
	// Read one identifier token.
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return rest[:i]
		}
	}
	return rest
}

// joinChainTables returns the chain's table names in order.
func (a *App) joinChainTables() []string {
	out := make([]string, len(a.joinChain))
	for i, s := range a.joinChain {
		out[i] = s.tableName
	}
	return out
}

// nextJoinAlias returns the alias to use for the next JOIN step (a, b, c, …).
func (a *App) nextJoinAlias() string {
	return string(rune('a' + len(a.joinChain)))
}

// loadViewItems fetches saved views from the store as TUI items. Errors are
// surfaced via the status bar and the modal opens with an empty list.
func (a *App) loadViewItems() []tui.ViewItem {
	if a.viewsStore == nil {
		return nil
	}
	saved, err := a.viewsStore.Load()
	if err != nil {
		a.log.Warn("views load failed", "err", err)
		return nil
	}
	items := make([]tui.ViewItem, len(saved))
	for i, v := range saved {
		items[i] = tui.ViewItem{Name: v.Name, SQL: v.SQL}
	}
	return items
}

// refreshSqlBarSchema rebuilds the SQL bar's autocomplete pools from the
// current schema cache. Called after introspection completes.
func (a *App) refreshSqlBarSchema() {
	summaries := a.cache.Tables()
	tables := make([]string, len(summaries))
	cols := make(map[string][]string, len(summaries))
	for i, ts := range summaries {
		name := ts.Name
		tables[i] = name
		if t := a.cache.Table(name); t != nil {
			colNames := make([]string, len(t.Columns))
			for j, c := range t.Columns {
				colNames[j] = c.Name
			}
			cols[name] = colNames
		}
	}
	a.sqlBar.SetSchema(tables, cols)
	a.sqlEditor.SetSchema(tables, cols)
}

// refreshCurrentTable re-queries the table currently shown in the data viewer.
// Returns nil when there is no active table or driver.
func (a *App) refreshCurrentTable() tea.Cmd {
	tbl := a.dataViewer.Table()
	if a.drv == nil || tbl == nil {
		return nil
	}
	return a.queryCmd(tbl, 0, 50)
}

// testConnResultMsg carries the outcome of an add-connection test back to
// Update so the modal can be dismissed (success) or annotated (failure).
type testConnResultMsg struct {
	conn config.Connection
	err  error
}

// testConnectionCmd runs Connect + Ping against the proposed config in a
// fresh driver, with a 5-second timeout. The driver is closed after the
// probe regardless of outcome. The DSN is taken as-is from the form (with
// the user-supplied password) — the keyring split happens AFTER a
// successful test, in persistConnection.
func (a *App) testConnectionCmd(conn config.Connection) tea.Cmd {
	return func() tea.Msg {
		drv, err := db.New(conn.Engine)
		if err != nil {
			return testConnResultMsg{conn: conn, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := drv.Connect(ctx, conn.DSN); err != nil {
			return testConnResultMsg{conn: conn, err: err}
		}
		if err := drv.Ping(ctx); err != nil {
			_ = drv.Close()
			return testConnResultMsg{conn: conn, err: err}
		}
		_ = drv.Close()
		return testConnResultMsg{conn: conn, err: nil}
	}
}

// persistConnection prepares a connection for storage. There are two paths
// depending on how the password was provided in the form:
//
//  1. Explicit password field (a.pendingPassword non-empty): the DSN was
//     typed without a password. We inject the literal `{password}` placeholder
//     and store the password in the keyring.
//  2. No explicit password (a.pendingPassword empty): the DSN may contain
//     an embedded password — split it via SplitDSN. SQLite and passwordless
//     URLs have nothing to extract.
//
// Returns the connection that should actually be saved to TOML. The caller
// is expected to clear a.pendingPassword and a.pendingTemplateDSN after.
func (a *App) persistConnection(conn config.Connection) (config.Connection, error) {
	if a.pendingPassword != "" {
		template, err := secrets.InjectPlaceholder(conn.Engine, a.pendingTemplateDSN)
		if err != nil {
			return conn, fmt.Errorf("template build: %w", err)
		}
		keyringKey := secrets.KeyringKeyFor(conn.Name)
		if err := secrets.SetPassword(keyringKey, a.pendingPassword); err != nil {
			return conn, fmt.Errorf("OS keyring not available: %v\n\nPlaintext fallback is intentionally NOT used. Workarounds:\n  • Start your OS keyring service (gnome-keyring / KWallet on Linux)\n  • Or edit %s by hand to use:\n      dsn_env = \"YOURVAR\"\n    and export YOURVAR='<full DSN>' in your shell", err, a.configPath)
		}
		return config.Connection{
			Name:       conn.Name,
			Engine:     conn.Engine,
			DSN:        template,
			KeyringKey: keyringKey,
		}, nil
	}

	template, password, has := secrets.SplitDSN(conn.Engine, conn.DSN)
	if !has {
		return conn, nil
	}
	keyringKey := secrets.KeyringKeyFor(conn.Name)
	if err := secrets.SetPassword(keyringKey, password); err != nil {
		return conn, fmt.Errorf("OS keyring not available: %v\n\nPlaintext fallback is intentionally NOT used. Workarounds:\n  • Start your OS keyring service (gnome-keyring / KWallet on Linux)\n  • Or edit %s by hand to use:\n      dsn_env = \"YOURVAR\"\n    and export YOURVAR='<full DSN>' in your shell", err, a.configPath)
	}
	return config.Connection{
		Name:       conn.Name,
		Engine:     conn.Engine,
		DSN:        template,
		KeyringKey: keyringKey,
	}, nil
}

// copyRowsTSV builds a TSV-formatted clipboard payload for the marked rows
// (or the row under the cursor when no marks are set). Returns the formatted
// string and the row count that fed it. Tabs and newlines inside cell values
// are replaced with spaces so the output stays well-formed for paste targets
// like spreadsheets.
func (a *App) copyRowsTSV() (string, int) {
	rows := a.dataViewer.MarkedRows()
	if len(rows) == 0 {
		if r, ok := a.dataViewer.SelectedRow(); ok {
			rows = []int{r}
		} else {
			return "", 0
		}
	}
	headers := a.dataViewer.ColumnNames()
	if len(headers) == 0 {
		return "", 0
	}
	var sb strings.Builder
	sb.WriteString(strings.Join(headers, "\t"))
	sb.WriteByte('\n')
	for _, idx := range rows {
		vals := a.dataViewer.RowValues(idx)
		for i, v := range vals {
			v = strings.ReplaceAll(v, "\t", " ")
			v = strings.ReplaceAll(v, "\r", " ")
			v = strings.ReplaceAll(v, "\n", " ")
			vals[i] = v
		}
		sb.WriteString(strings.Join(vals, "\t"))
		sb.WriteByte('\n')
	}
	return sb.String(), len(rows)
}

// renderTabBar produces the single-line tab strip rendered above the
// schema browser / data viewer body. Returns "" when no tabs exist
// (e.g. before the first connection). Active tab gets a brighter
// highlight; the schema tab keeps a fixed leftmost position.
func (a *App) renderTabBar() string {
	if len(a.tabs) == 0 {
		return ""
	}
	activeStyle := lipgloss.NewStyle().
		Foreground(tui.CtpBase).
		Background(tui.CtpMauve).
		Padding(0, 1).
		Bold(true)
	idleStyle := lipgloss.NewStyle().
		Foreground(tui.CtpSubtext1).
		Padding(0, 1)
	parts := make([]string, 0, len(a.tabs))
	for i, t := range a.tabs {
		title := t.Title
		if t.Kind == TabSchema {
			title = "Schema"
		}
		if i == a.activeTab {
			parts = append(parts, activeStyle.Render(title))
		} else {
			parts = append(parts, idleStyle.Render(title))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	if a.width > 0 {
		bar = lipgloss.NewStyle().MaxWidth(a.width).Render(bar)
	}
	return bar
}

// activeAIName returns the active profile's name, or "" when no AI is set.
func (a *App) activeAIName() string {
	if p := a.cfg.ActiveProfile(); p != nil {
		return p.Name
	}
	return ""
}

// openAIProfileList opens the AI Profile list modal so the user can
// switch between configured providers, add new ones, edit, or delete.
func (a *App) openAIProfileList() {
	a.aiProfiles = tui.NewAIProfileListModel(a.cfg.AIs, a.cfg.ActiveAI, a.width, a.height)
	a.modal = ModalAIProfileList
}

// openAI is the single entry point for invoking the AI feature, called
// from any screen that exposes Ctrl+A. When AI is already configured it
// jumps straight to the Ask Panel; when it isn't, it surfaces the setup
// wizard so the user never sees a dead-end "AI disabled" hint.
func (a *App) openAI() {
	if a.ai.Enabled() {
		a.screen = ScreenAskPanel
		a.askPanel = tui.NewAskPanelModel(true, a.width, a.height)
		return
	}
	a.aiSetup = tui.NewAISetupModel(a.width, a.height)
	a.modal = ModalAISetup
}

// buildAIDebugPrompt augments the original natural-language question
// with the prior failed SQL, its DB error, and the user's hint so the
// AI sees the full failure context. The prompt builder downstream
// already prepends the schema; this just substitutes the user payload.
func buildAIDebugPrompt(question, prevSQL, errMsg, hint string) string {
	var sb strings.Builder
	sb.WriteString(strings.TrimSpace(question))
	sb.WriteString("\n\nThe previous attempt failed with this error:\n")
	sb.WriteString(strings.TrimSpace(errMsg))
	sb.WriteString("\n\nThe SQL that failed was:\n")
	sb.WriteString(strings.TrimSpace(prevSQL))
	if h := strings.TrimSpace(hint); h != "" {
		sb.WriteString("\n\nUser hint: ")
		sb.WriteString(h)
	}
	sb.WriteString("\n\nPlease return a corrected SQL.")
	return sb.String()
}

// isReadOnlySQL reports whether sql is safe to auto-execute without user
// confirmation. The check is intentionally conservative: only statements
// that begin with SELECT / WITH / EXPLAIN / SHOW / PRAGMA / DESC[RIBE]
// (after stripping leading comments and whitespace) qualify. Anything
// else — even if benign in the user's mind — falls back to the manual
// preview-and-confirm flow so the AI cannot mutate data on its own.
func isReadOnlySQL(sql string) bool {
	s := strings.TrimSpace(sql)
	// Strip any leading line comments (-- ...) and block comments (/* ... */).
	for {
		switch {
		case strings.HasPrefix(s, "--"):
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = strings.TrimSpace(s[i+1:])
			} else {
				return false
			}
		case strings.HasPrefix(s, "/*"):
			if i := strings.Index(s, "*/"); i >= 0 {
				s = strings.TrimSpace(s[i+2:])
			} else {
				return false
			}
		default:
			goto done
		}
	}
done:
	up := strings.ToUpper(s)
	for _, prefix := range []string{"SELECT ", "SELECT\n", "SELECT\t", "WITH ", "EXPLAIN ", "SHOW ", "PRAGMA ", "DESCRIBE ", "DESC "} {
		if strings.HasPrefix(up, prefix) {
			return true
		}
	}
	// SELECT alone (no trailing space) — accept too.
	if up == "SELECT" || up == "EXPLAIN" || up == "SHOW" {
		return true
	}
	return false
}

// deleteAIProfile removes a profile by name from the config, drops the
// keyring secret (best-effort), persists the result, and re-resolves
// the active provider so it points at the next remaining profile (or
// becomes NoOp when the list is empty).
func (a *App) deleteAIProfile(name string) tea.Cmd {
	idx := -1
	var keyringKey string
	for i, p := range a.cfg.AIs {
		if p.Name == name {
			idx = i
			keyringKey = p.KeyringKey
			break
		}
	}
	if idx < 0 {
		return a.statusBar.SetErr(fmt.Errorf("[config] AI profile %q not found", name))
	}
	a.cfg.AIs = append(a.cfg.AIs[:idx], a.cfg.AIs[idx+1:]...)
	if a.cfg.ActiveAI == name {
		if len(a.cfg.AIs) > 0 {
			a.cfg.ActiveAI = a.cfg.AIs[0].Name
		} else {
			a.cfg.ActiveAI = ""
		}
	}
	if keyringKey != "" {
		if err := secrets.DeletePassword(keyringKey); err != nil {
			a.log.Warn("ai keyring delete failed", "key", keyringKey, "err", err)
		}
	}
	cmd := a.saveConfigAnnotated("AI profile deleted: " + name)
	a.ai = ai.New(resolveAIConfig(a.cfg.ActiveProfile()))
	return cmd
}

// keyringKeyForProfile returns the OS-keyring entry path used to store
// an AI profile's API key. Per-profile namespacing keeps multiple
// profiles' secrets isolated and lets us delete one without affecting
// the others.
func keyringKeyForProfile(name string) string {
	return "zdb/ai-key/" + name
}

// resolveAIConfig translates an AI profile into the runtime ai.Config
// the provider needs, resolving the API key from the OS keyring or env
// var (in that order). Returns a zero ai.Config when p is nil — ai.New
// treats that as "disabled" and returns NoOp.
func resolveAIConfig(p *config.AIProfile) ai.Config {
	if p == nil {
		return ai.Config{}
	}
	cfg := ai.Config{
		ProfileName:    p.Name,
		BaseURL:        p.BaseURL,
		Model:          p.Model,
		TimeoutSeconds: p.TimeoutSeconds,
	}
	if p.KeyringKey != "" {
		if pw, err := secrets.GetPassword(p.KeyringKey); err == nil {
			cfg.APIKey = pw
		}
	}
	if cfg.APIKey == "" && p.APIKeyEnv != "" {
		cfg.APIKey = os.Getenv(p.APIKeyEnv)
	}
	return cfg
}

// openSQLEditor switches into the full-screen SQL editor, pre-filling it
// with the last-run SQL so the user can iterate on a prior query without
// retyping. Initializes the model fresh each time so the textarea picks
// up the current terminal size.
func (a *App) openSQLEditor() {
	a.sqlEditor = tui.NewSQLEditorModel(a.width, a.height)
	a.refreshSqlBarSchema() // also feeds the editor's autocomplete pools
	if a.lastSQL != "" {
		a.sqlEditor.SetValue(a.lastSQL)
	}
	a.screen = ScreenSQLEditor
}

// isFilterClause reports whether the input looks like a SQL clause that
// should be appended to an existing query (a filter) rather than run as
// a standalone statement. Recognized starts: WHERE, ORDER BY, GROUP BY,
// HAVING, LIMIT — case- and whitespace-insensitive.
func isFilterClause(sql string) bool {
	up := strings.ToUpper(strings.TrimSpace(sql))
	for _, kw := range []string{"WHERE ", "ORDER BY ", "GROUP BY ", "HAVING ", "LIMIT "} {
		if strings.HasPrefix(up, kw) {
			return true
		}
	}
	return false
}

// refreshSqlBarPlaceholder sets the SQL bar's placeholder hint based on
// whether a JOIN chain is currently materialized. In JOIN view, the bar
// can either filter the active result (clause input) or replace it with
// a fresh query — the placeholder advertises that branching behavior.
func (a *App) refreshSqlBarPlaceholder() {
	if len(a.joinChain) >= 2 && a.lastSQL != "" {
		a.sqlBar.SetPlaceholder("WHERE/ORDER BY/… filters the JOIN · full SELECT replaces it")
	} else {
		a.sqlBar.SetPlaceholder("press : to enter SQL · Enter run · Esc unfocus")
	}
}

// editTestConnResultMsg carries the outcome of an edit-connection test back
// to Update so the modal can be dismissed (success) or annotated (failure).
// Distinct from testConnResultMsg so the add and edit paths don't share state.
type editTestConnResultMsg struct {
	conn config.Connection
	err  error
}

// testEditConnectionCmd is the edit-flow counterpart of testConnectionCmd.
// Same probe semantics — a fresh driver, Connect+Ping, 5s timeout — but
// emits editTestConnResultMsg.
func (a *App) testEditConnectionCmd(conn config.Connection) tea.Cmd {
	return func() tea.Msg {
		drv, err := db.New(conn.Engine)
		if err != nil {
			return editTestConnResultMsg{conn: conn, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := drv.Connect(ctx, conn.DSN); err != nil {
			return editTestConnResultMsg{conn: conn, err: err}
		}
		if err := drv.Ping(ctx); err != nil {
			_ = drv.Close()
			return editTestConnResultMsg{conn: conn, err: err}
		}
		_ = drv.Close()
		return editTestConnResultMsg{conn: conn, err: nil}
	}
}

// persistEditedConnection prepares an edited connection for storage.
//
// If passwordChanged is true, the literal password is stored in the OS
// keyring (reusing original.KeyringKey when present, otherwise deriving a
// new key from the updated name) and the DSN is rewritten to a template
// containing the {password} placeholder. If passwordChanged is false, the
// original.KeyringKey is preserved as-is — no secret rotation happens.
//
// The caller is responsible for clearing the pendingEdit* fields after.
func (a *App) persistEditedConnection(original, updated config.Connection, password string, passwordChanged bool) (config.Connection, error) {
	if !passwordChanged {
		// Keep KeyringKey from original (form already copies it, but be explicit).
		updated.KeyringKey = original.KeyringKey
		return updated, nil
	}
	template, err := secrets.InjectPlaceholder(updated.Engine, updated.DSN)
	if err != nil {
		// DSN may already contain {password} or be password-less (e.g. SQLite).
		// Fall back to the raw DSN.
		template = updated.DSN
	}
	keyringKey := original.KeyringKey
	if keyringKey == "" {
		keyringKey = secrets.KeyringKeyFor(updated.Name)
	}
	if err := secrets.SetPassword(keyringKey, password); err != nil {
		return updated, fmt.Errorf("OS keyring not available: %v\n\nPlaintext fallback is intentionally NOT used. Workarounds:\n  • Start your OS keyring service (gnome-keyring / KWallet on Linux)\n  • Or edit %s by hand to use:\n      dsn_env = \"YOURVAR\"\n    and export YOURVAR='<full DSN>' in your shell", err, a.configPath)
	}
	return config.Connection{
		Name:       updated.Name,
		Engine:     updated.Engine,
		DSN:        template,
		KeyringKey: keyringKey,
		DSNEnv:     updated.DSNEnv,
	}, nil
}

// deleteConnection removes a connection by name from cfg, persists the new
// config, and best-effort removes its keyring secret. When the deletion
// leaves the list empty, the screen swings back to the welcome state.
// Returns a tea.Cmd that updates the status bar.
func (a *App) deleteConnection(name string) tea.Cmd {
	var idx = -1
	var removed config.Connection
	for i, c := range a.cfg.Connections {
		if c.Name == name {
			idx = i
			removed = c
			break
		}
	}
	if idx < 0 {
		return a.statusBar.SetErr(fmt.Errorf("[config] connection %q not found", name))
	}
	a.cfg.Connections = append(a.cfg.Connections[:idx], a.cfg.Connections[idx+1:]...)

	// Best-effort: drop the keyring secret. We don't fail the delete if the
	// keyring is unavailable — the TOML entry is gone, which is the user's
	// intent.
	if removed.KeyringKey != "" {
		if err := secrets.DeletePassword(removed.KeyringKey); err != nil {
			a.log.Warn("keyring delete failed", "key", removed.KeyringKey, "err", err)
		}
	}

	cmd := a.saveConfigAnnotated("connection deleted: " + name)

	if len(a.cfg.Connections) == 0 {
		a.screen = ScreenWelcome
		a.welcome = tui.NewWelcomeModel(a.width, a.height)
	} else if a.screen == ScreenConnPicker {
		a.connPicker = tui.NewConnPickerModel(a.cfg.Connections, a.width, a.height)
	}
	return cmd
}

// reconnectCmd attempts to reconnect the active driver.
func (a *App) reconnectCmd() tea.Cmd {
	if a.drv == nil || len(a.cfg.Connections) == 0 {
		return nil
	}
	// Find the active connection config by name
	for _, conn := range a.cfg.Connections {
		if conn.Name == a.connName {
			return a.connectCmd(conn)
		}
	}
	return nil
}

// execSQLCmd executes a raw SQL statement. SELECTs update the data viewer;
// non-SELECTs run in a transaction and require confirmation.
func (a *App) execSQLCmd(sql string) tea.Cmd {
	if a.drv == nil {
		return a.statusBar.SetErr(fmt.Errorf("[db] not connected"))
	}
	drv := a.drv
	reqID := newReqID()
	ctx, cancel := context.WithCancel(context.Background())
	a.inflight[reqID] = cancel

	return func() tea.Msg {
		rs, err := drv.Query(ctx, sql)
		if err != nil {
			return ErrMsg{Source: "db", Err: err}
		}
		return DBQueryDoneMsg{ReqID: reqID, ResultSet: rs}
	}
}

// askAICmd submits a natural-language question to the AI provider.
func (a *App) askAICmd(question string) tea.Cmd {
	if !a.ai.Enabled() {
		return a.statusBar.SetMsg("configure AI to enable")
	}
	reqID := newReqID()
	ctx, cancel := context.WithCancel(context.Background())
	a.inflight[reqID] = cancel

	schema := a.cache.Get()
	provider := a.ai

	return func() tea.Msg {
		sql, err := provider.Ask(ctx, schema, question)
		return AIAskDoneMsg{ReqID: reqID, SQL: sql, Err: err}
	}
}

// saveConfigAnnotated persists a.cfg to a.configPath using the backup-aware
// SaveWithBackupStatus API and returns a tea.Cmd that updates the status bar.
// On a successful write it emits successMsg, suffixed with " (backup skipped)"
// when the .bak refresh failed. On a hard write failure it surfaces the error
// via SetErr. successMsg must be non-empty.
//
// When the config file was modified externally since load, the write is aborted
// and the status bar shows a reconcile hint pointing the user to
// `zdb config import` or a restart. The user's in-memory state is NOT discarded.
//
// Do not generalize this helper — it is intentionally scoped to the single
// concern of config-save + status-bar feedback (R7 mitigation).
func (a *App) saveConfigAnnotated(successMsg string) tea.Cmd {
	if a.configPath == "" {
		return a.statusBar.SetErr(fmt.Errorf("[config] no path resolved"))
	}
	newSnap, backupErr, writeErr := config.SaveWithBackupStatus(a.cfg, a.configPath, a.snapshot)
	if errors.Is(writeErr, config.ErrConfigChangedExternally) {
		// File was modified externally since load. Surface the reconcile hint.
		// The user's in-memory state is preserved; they can restart zdb or run
		// `zdb config import` to reconcile.
		return a.statusBar.SetErr(fmt.Errorf(
			"[config] file changed externally — close and reopen zdb, or run `zdb config import` to reconcile",
		))
	}
	if writeErr != nil {
		return a.statusBar.SetErr(fmt.Errorf("[config] save: %v", writeErr))
	}
	// Refresh the snapshot so subsequent saves do not trigger a false stale error.
	a.snapshot = newSnap
	if errors.Is(backupErr, config.ErrBackupSkipped) {
		return a.statusBar.SetMsg(successMsg + " (backup skipped)")
	}
	return a.statusBar.SetMsg(successMsg)
}

// pingCmd pings the active driver.
func (a *App) pingCmd() tea.Cmd {
	if a.drv == nil {
		return nil
	}
	drv := a.drv
	connName := a.connName

	return func() tea.Msg {
		if err := drv.Ping(context.Background()); err != nil {
			return ConnLostMsg{ConnName: connName}
		}
		return nil
	}
}

