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
