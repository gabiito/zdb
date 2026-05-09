package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gabiito/db-viewer/internal/ai"
	"github.com/gabiito/db-viewer/internal/config"
	"github.com/gabiito/db-viewer/internal/db"
	"github.com/gabiito/db-viewer/internal/tui"
)

// App is the Bubbletea root model for db-viewer.
type App struct {
	cfg  config.Config
	log  *slog.Logger
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
	sqlPanel     tui.SqlPanelModel
	askPanel     tui.AskPanelModel
	confirm      tui.ConfirmModel
	statusBar    tui.StatusBarModel
	spinner      spinner.Model

	// Current active connection name (for messages)
	connName string

	// In-flight request tracking: reqID → cancel function
	inflight map[string]context.CancelFunc

	// Open transaction (during confirm flow)
	pendingTx db.Tx

	// Terminal dimensions
	width, height int
}

// NewApp creates a new App from config.
func NewApp(cfg config.Config, log *slog.Logger) *App {
	s := spinner.New()
	s.Spinner = spinner.Dot

	aiCfg := ai.Config{}
	if cfg.AI != nil {
		aiCfg.BaseURL = cfg.AI.BaseURL
		aiCfg.Model = cfg.AI.Model
		aiCfg.TimeoutSeconds = cfg.AI.TimeoutSeconds
	}

	provider := ai.New(aiCfg)

	return &App{
		cfg:      cfg,
		log:      log,
		ai:       provider,
		edit:     &EditBuffer{},
		exec:     &Executor{},
		screen:   ScreenConnPicker,
		inflight: make(map[string]context.CancelFunc),
		spinner:  s,
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.spinner.Tick,
		// Status bar will be initialized on first WindowSizeMsg
	)
}

// Update implements tea.Model — the central event dispatcher.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Global: always handle window size
	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		a.width = wsMsg.Width
		a.height = wsMsg.Height
		a.statusBar.SetWidth(wsMsg.Width)

		// Re-initialize conn picker if needed
		if a.screen == ScreenConnPicker {
			a.connPicker = tui.NewConnPickerModel(a.cfg.Connections, a.width, a.height)
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
		cmds = append(cmds, a.connectCmd(msg.Conn))

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
			a.screen = ScreenSchemaBrowser
			a.schemaBrow = tui.NewSchemaBrowserModel(a.cache.Tables(), &a.cache, a.width, a.height)
		}

	case tui.OpenTableMsg:
		if msg.Table != nil {
			a.screen = ScreenDataViewer
			pageSize := 50
			a.dataViewer = tui.NewDataViewerModel(msg.Table, pageSize, a.width, a.height)
			// Load initial data
			cmds = append(cmds, a.queryCmd(msg.Table, 0, pageSize))
		}

	case DBQueryDoneMsg:
		if _, live := a.inflight[msg.ReqID]; !live {
			break
		}
		delete(a.inflight, msg.ReqID)
		if msg.Err != nil {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[db] query: %v", msg.Err)))
		} else {
			a.dataViewer.SetData(msg.ResultSet)
		}

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
		if a.pendingTx != nil {
			cmds = append(cmds, a.commitCmd(a.pendingTx))
			a.pendingTx = nil
		}
		a.modal = ModalNone

	case tui.ConfirmNoMsg:
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

	case tui.SqlExecuteMsg:
		cmds = append(cmds, a.execSQLCmd(msg.SQL))
		a.screen = ScreenDataViewer

	case tui.AskSubmitMsg:
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
		if msg.Err != nil {
			cmds = append(cmds, a.statusBar.SetErr(fmt.Errorf("[ai] ask: %v", msg.Err)))
		} else {
			a.askPanel.SetPreview(msg.SQL)
			if msg.Truncated {
				cmds = append(cmds, a.statusBar.SetMsg("AI: schema truncated to 30 tables"))
			}
		}

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

	// Modal takes priority
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
		}
		return tea.Batch(cmds...)
	}

	// Active screen
	switch a.screen {
	case ScreenConnPicker:
		var cmd tea.Cmd
		a.connPicker, cmd = a.connPicker.Update(msg)
		cmds = append(cmds, cmd)
	case ScreenSchemaBrowser:
		var cmd tea.Cmd
		a.schemaBrow, cmd = a.schemaBrow.Update(msg)
		cmds = append(cmds, cmd)
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
			case ":": // open SQL panel
				a.screen = ScreenSqlPanel
				a.sqlPanel = tui.NewSqlPanelModel(a.width, a.height)
			case "ctrl+a", "f2": // open AI ask panel
				a.screen = ScreenAskPanel
				a.askPanel = tui.NewAskPanelModel(a.ai.Enabled(), a.width, a.height)
			case "v": // view cell
				if row, col, ok := a.dataViewer.SelectedCell(); ok {
					a.dataViewer.OpenCellView(row, col)
					a.modal = ModalCellView
				}
			case "enter": // edit cell
				if row, col, ok := a.dataViewer.SelectedCell(); ok {
					table := a.dataViewer.Table()
					if len(table.PKCols) == 0 {
						cmds = append(cmds, a.statusBar.SetMsg("table has no primary key — read-only"))
					} else {
						a.cellEdit = tui.NewCellEditModel(row, col, a.dataViewer.CellValue(row, col))
						a.modal = ModalCellEdit
					}
				}
			case "d": // delete row
				if row, ok := a.dataViewer.SelectedRow(); ok {
					table := a.dataViewer.Table()
					if len(table.PKCols) == 0 {
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
				// Cancel in-flight requests
				for id, cancel := range a.inflight {
					cancel()
					delete(a.inflight, id)
				}
			}
		}
	case ScreenSqlPanel:
		var cmd tea.Cmd
		a.sqlPanel, cmd = a.sqlPanel.Update(msg)
		cmds = append(cmds, cmd)
		if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" {
			if !a.sqlPanel.IsActive() {
				a.screen = ScreenDataViewer
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
	if a.width == 0 {
		return "Loading..."
	}

	var body string

	switch a.screen {
	case ScreenConnPicker:
		body = a.connPicker.View()
	case ScreenSchemaBrowser:
		body = a.schemaBrow.View()
	case ScreenDataViewer:
		body = a.dataViewer.View()
	case ScreenSqlPanel:
		body = a.sqlPanel.View()
	case ScreenAskPanel:
		body = a.askPanel.View()
	}

	// Modal overlay
	if a.modal != ModalNone {
		var overlay string
		switch a.modal {
		case ModalCellEdit:
			overlay = a.cellEdit.View()
		case ModalConfirm:
			overlay = a.confirm.View()
		case ModalCellView:
			overlay = a.dataViewer.CellViewView()
		}
		body = renderOverlay(body, overlay, a.width, a.height)
	}

	// Spinner when in-flight
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

	return lipgloss.JoinVertical(lipgloss.Left, body, statusLine)
}

// renderOverlay places an overlay string on top of the base view.
func renderOverlay(base, overlay string, width, height int) string {
	// Simple implementation: render overlay below base content
	// A more sophisticated version would center it
	_ = width
	_ = height
	return base + "\n" + overlay
}

// ---- Async command builders ----

// connectCmd connects to the given connection profile.
func (a *App) connectCmd(conn config.Connection) tea.Cmd {
	reqID := newReqID()
	ctx, cancel := context.WithCancel(context.Background())
	a.inflight[reqID] = cancel

	// Find or create a driver
	drv, err := db.New(conn.Engine)
	if err != nil {
		cancel()
		delete(a.inflight, reqID)
		return a.statusBar.SetErr(fmt.Errorf("[config] %s: %v", conn.Name, err))
	}
	a.drv = drv

	return func() tea.Msg {
		err := drv.Connect(ctx, conn.DSN)
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

// commitCmd commits a pending transaction.
func (a *App) commitCmd(tx db.Tx) tea.Cmd {
	return func() tea.Msg {
		if err := tx.Commit(context.Background()); err != nil {
			return ErrMsg{Source: "db", Err: err}
		}
		return DBExecDoneMsg{ReqID: "", RowsAffected: 1}
	}
}

// rollbackCmd rolls back a pending transaction.
func (a *App) rollbackCmd(tx db.Tx) tea.Cmd {
	return func() tea.Msg {
		if err := tx.Rollback(context.Background()); err != nil {
			return ErrMsg{Source: "db", Err: err}
		}
		return DBExecDoneMsg{ReqID: "", RowsAffected: 0}
	}
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

