package core

// ScreenID identifies the active full-screen view.
type ScreenID int

const (
	ScreenConnPicker ScreenID = iota
	ScreenSchemaBrowser
	ScreenDataViewer
	ScreenSqlPanel
	ScreenAskPanel
)

// Modal identifies an active modal overlay (rendered above the active screen).
type Modal int

const (
	ModalNone Modal = iota
	ModalCellEdit
	ModalConfirm // mutating SQL / DELETE / commit-edit confirmation
	ModalNotice  // non-blocking info banner
	ModalCellView // cell viewport (read-only, triggered by 'v')
	ModalStagedView // list of staged edits (triggered by 'S')
	ModalJoinWizard // multi-step join builder (triggered by 'J')
	ModalViewsList // list of saved views (triggered by 'V')
	ModalSaveView // textinput to name a saved view (triggered by 'W')
	ModalJoinChoice // add-vs-replace prompt when J is pressed on a join chain
	ModalAddConnection // form for adding a new DB connection from the conn picker
)

// FocusState tracks which sub-component owns keyboard input within a screen.
type FocusState int

const (
	FocusDefault     FocusState = iota
	FocusEditor                 // textinput or textarea in SQL/Ask panel
	FocusSuggestions            // suggestion list within SQL panel
	FocusTable                  // bubble-table data viewer
	FocusConfirm                // confirmation banner
	FocusCellView               // cell viewport
)
