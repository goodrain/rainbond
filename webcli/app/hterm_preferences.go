// Copyright (C) 2014-2018 Goodrain Co., Ltd.
// RAINBOND, Application Management Platform

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version. For any non-GPL usage of Rainbond,
// one or multiple Commercial Licenses authorized by Goodrain Co., Ltd.
// must be obtained first.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package app

type HtermPrefernces struct {
	AltGrMode                     *string                      `hcl:"alt_gr_mode"`
	AltBackspaceIsMetaBackspace   bool                         `hcl:"alt_backspace_is_meta_backspace"`
	AltIsMeta                     bool                         `hcl:"alt_is_meta"`
	AltSendsWhat                  string                       `hcl:"alt_sends_what"`
	AudibleBellSound              string                       `hcl:"audible_bell_sound"`
	DesktopNotificationBell       bool                         `hcl:"desktop_notification_bell"`
	BackgroundColor               string                       `hcl:"background_color"`
	BackgroundImage               string                       `hcl:"background_image"`
	BackgroundSize                string                       `hcl:"background_size"`
	BackgroundPosition            string                       `hcl:"background_position"`
	BackspaceSendsBackspace       bool                         `hcl:"backspace_sends_backspace"`
	CharacterMapOverrides         map[string]map[string]string `hcl:"character_map_overrides"`
	CloseOnExit                   bool                         `hcl:"close_on_exit"`
	CursorBlink                   bool                         `hcl:"cursor_blink"`
	CursorBlinkCycle              [2]int                       `hcl:"cursor_blink_cycle"`
	CursorColor                   string                       `hcl:"cursor_color"`
	ColorPaletteOverrides         []*string                    `hcl:"color_palette_overrides"`
	CopyOnSelect                  bool                         `hcl:"copy_on_select"`
	UseDefaultWindowCopy          bool                         `hcl:"use_default_window_copy"`
	ClearSelectionAfterCopy       bool                         `hcl:"clear_selection_after_copy"`
	CtrlPlusMinusZeroZoom         bool                         `hcl:"ctrl_plus_minus_zero_zoom"`
	CtrlCCopy                     bool                         `hcl:"ctrl_c_copy"`
	CtrlVPaste                    bool                         `hcl:"ctrl_v_paste"`
	EastAsianAmbiguousAsTwoColumn bool                         `hcl:"east_asian_ambiguous_as_two_column"`
	Enable8BitControl             *bool                        `hcl:"enable_8_bit_control"`
	EnableBold                    *bool                        `hcl:"enable_bold"`
	EnableBoldAsBright            bool                         `hcl:"enable_bold_as_bright"`
	EnableClipboardNotice         bool                         `hcl:"enable_clipboard_notice"`
	EnableClipboardWrite          bool                         `hcl:"enable_clipboard_write"`
	EnableDec12                   bool                         `hcl:"enable_dec12"`
	Environment                   map[string]string            `hcl:"environment"`
	FontFamily                    string                       `hcl:"font_family"`
	FontSize                      int                          `hcl:"font_size"`
	FontSmoothing                 string                       `hcl:"font_smoothing"`
	ForegroundColor               string                       `hcl:"foreground_color"`
	HomeKeysScroll                bool                         `hcl:"home_keys_scroll"`
	Keybindings                   map[string]string            `hcl:"keybindings"`
	MaxStringSequence             int                          `hcl:"max_string_sequence"`
	MediaKeysAreFkeys             bool                         `hcl:"media_keys_are_fkeys"`
	MetaSendsEscape               bool                         `hcl:"meta_sends_escape"`
	MousePasteButton              *int                         `hcl:"mouse_paste_button"`
	PageKeysScroll                bool                         `hcl:"page_keys_scroll"`
	PassAltNumber                 *bool                        `hcl:"pass_alt_number"`
	PassCtrlNumber                *bool                        `hcl:"pass_ctrl_number"`
	PassMetaNumber                *bool                        `hcl:"pass_meta_number"`
	PassMetaV                     bool                         `hcl:"pass_meta_v"`
	ReceiveEncoding               string                       `hcl:"receive_encoding"`
	ScrollOnKeystroke             bool                         `hcl:"scroll_on_keystroke"`
	ScrollOnOutput                bool                         `hcl:"scroll_on_output"`
	ScrollbarVisible              bool                         `hcl:"scrollbar_visible"`
	ScrollWheelMoveMultiplier     int                          `hcl:"scroll_wheel_move_multiplier"`
	SendEncoding                  string                       `hcl:"send_encoding"`
	ShiftInsertPaste              bool                         `hcl:"shift_insert_paste"`
	UserCss                       string                       `hcl:"user_css"`
}
