package tui

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"surge/internal/config"

	"github.com/charmbracelet/lipgloss"
)

// viewSettings renders the Btop-style settings page
func (m RootModel) viewSettings() string {
	width := m.width - 4
	height := m.height - 4
	if width < 80 {
		width = 80
	}
	if height < 20 {
		height = 20
	}

	// Get category metadata
	categories := config.CategoryOrder()
	metadata := config.GetSettingsMetadata()

	// === TAB BAR ===
	var tabItems []string
	for i, cat := range categories {
		label := fmt.Sprintf("[%d] %s", i+1, cat)
		if i == m.SettingsActiveTab {
			tabItems = append(tabItems, ActiveTabStyle.Render(label))
		} else {
			tabItems = append(tabItems, TabStyle.Render(label))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Left, tabItems...)

	// === CONTENT AREA ===
	currentCategory := categories[m.SettingsActiveTab]
	settingsMeta := metadata[currentCategory]

	// Calculate column widths
	innerWidth := width - 4
	leftWidth := int(float64(innerWidth) * 0.40)
	rightWidth := innerWidth - leftWidth - 3 // -3 for separator

	// Get current settings values
	settingsValues := m.getSettingsValues(currentCategory)

	// === LEFT COLUMN: Settings List ===
	var listLines []string
	for i, meta := range settingsMeta {
		value := settingsValues[meta.Key]

		// Format key and value
		keyStr := meta.Label
		valueStr := formatSettingValue(value, meta.Type)

		// Calculate dots between key and value
		dotsCount := leftWidth - lipgloss.Width(keyStr) - lipgloss.Width(valueStr) - 4
		if dotsCount < 2 {
			dotsCount = 2
		}
		dots := strings.Repeat(".", dotsCount)

		line := fmt.Sprintf("%s %s %s", keyStr, dots, valueStr)

		// Highlight selected row
		if i == m.SettingsSelectedRow {
			if m.SettingsIsEditing {
				// Show input field instead of value
				editLine := fmt.Sprintf("%s: %s", keyStr, m.SettingsInput.View())
				line = lipgloss.NewStyle().
					Foreground(ColorNeonCyan).
					Render(editLine)
			} else {
				line = lipgloss.NewStyle().
					Foreground(ColorNeonPink).
					Bold(true).
					Render("> " + line)
			}
		} else {
			line = lipgloss.NewStyle().
				Foreground(ColorLightGray).
				Render("  " + line)
		}

		listLines = append(listLines, line)
	}

	listContent := lipgloss.JoinVertical(lipgloss.Left, listLines...)
	listBox := lipgloss.NewStyle().
		Width(leftWidth).
		Height(height - 8). // Account for tabs and help
		Render(listContent)

	// === RIGHT COLUMN: Description ===
	var description string
	if m.SettingsSelectedRow < len(settingsMeta) {
		meta := settingsMeta[m.SettingsSelectedRow]
		description = lipgloss.NewStyle().
			Foreground(ColorLightGray).
			Width(rightWidth - 2).
			Render(meta.Description)
	}

	// Add current value display in description
	if m.SettingsSelectedRow < len(settingsMeta) {
		meta := settingsMeta[m.SettingsSelectedRow]
		value := settingsValues[meta.Key]
		valueDisplay := lipgloss.NewStyle().
			Foreground(ColorNeonCyan).
			Bold(true).
			Render(fmt.Sprintf("\n\nCurrent: %s", formatSettingValue(value, meta.Type)))
		description += valueDisplay
	}

	descBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorGray).
		Padding(1).
		Width(rightWidth).
		Height(height - 8).
		Render(description)

	// === SEPARATOR ===
	separator := lipgloss.NewStyle().
		Foreground(ColorGray).
		Render(" │ ")

	// === COMBINE COLUMNS ===
	content := lipgloss.JoinHorizontal(lipgloss.Top, listBox, separator, descBox)

	// === HELP TEXT ===
	helpText := lipgloss.NewStyle().
		Foreground(ColorGray).
		Render("[↑/↓] Navigate  [Enter] Edit/Toggle  [1-4] Category  [Esc] Close")

	// === FINAL ASSEMBLY ===
	fullContent := lipgloss.JoinVertical(lipgloss.Left,
		"",
		tabBar,
		"",
		content,
		"",
		helpText,
	)

	paddedContent := lipgloss.NewStyle().Padding(0, 2).Render(fullContent)
	box := renderBtopBox("Settings", paddedContent, width, height, ColorNeonPink, false)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// getSettingsValues returns a map of setting key -> value for a category
func (m RootModel) getSettingsValues(category string) map[string]interface{} {
	values := make(map[string]interface{})

	switch category {
	case "General":
		values["default_download_dir"] = m.Settings.General.DefaultDownloadDir
		values["warn_on_duplicate"] = m.Settings.General.WarnOnDuplicate
		values["extension_prompt"] = m.Settings.General.ExtensionPrompt
		values["auto_resume"] = m.Settings.General.AutoResume
	case "Connections":
		values["max_connections_per_host"] = m.Settings.Connections.MaxConnectionsPerHost
		values["max_global_connections"] = m.Settings.Connections.MaxGlobalConnections
		values["user_agent"] = m.Settings.Connections.UserAgent
	case "Chunks":
		values["min_chunk_size"] = m.Settings.Chunks.MinChunkSize
		values["max_chunk_size"] = m.Settings.Chunks.MaxChunkSize
		values["target_chunk_size"] = m.Settings.Chunks.TargetChunkSize
		values["worker_buffer_size"] = m.Settings.Chunks.WorkerBufferSize
	case "Performance":
		values["max_task_retries"] = m.Settings.Performance.MaxTaskRetries
		values["slow_worker_threshold"] = m.Settings.Performance.SlowWorkerThreshold
		values["slow_worker_grace_period"] = m.Settings.Performance.SlowWorkerGracePeriod
		values["stall_timeout"] = m.Settings.Performance.StallTimeout
		values["speed_ema_alpha"] = m.Settings.Performance.SpeedEmaAlpha
	}

	return values
}

// setSettingValue sets a setting value from string input
func (m *RootModel) setSettingValue(category, key, value string) error {
	metadata := config.GetSettingsMetadata()
	metas := metadata[category]

	var meta config.SettingMeta
	for _, sm := range metas {
		if sm.Key == key {
			meta = sm
			break
		}
	}

	switch category {
	case "General":
		return m.setGeneralSetting(key, value, meta.Type)
	case "Connections":
		return m.setConnectionsSetting(key, value, meta.Type)
	case "Chunks":
		return m.setChunksSetting(key, value, meta.Type)
	case "Performance":
		return m.setPerformanceSetting(key, value, meta.Type)
	}

	return nil
}

func (m *RootModel) setGeneralSetting(key, value, typ string) error {
	switch key {
	case "default_download_dir":
		m.Settings.General.DefaultDownloadDir = value
	case "warn_on_duplicate":
		m.Settings.General.WarnOnDuplicate = !m.Settings.General.WarnOnDuplicate
	case "extension_prompt":
		m.Settings.General.ExtensionPrompt = !m.Settings.General.ExtensionPrompt
	case "auto_resume":
		m.Settings.General.AutoResume = !m.Settings.General.AutoResume
	}
	return nil
}

func (m *RootModel) setConnectionsSetting(key, value, typ string) error {
	switch key {
	case "max_connections_per_host":
		if v, err := strconv.Atoi(value); err == nil {
			m.Settings.Connections.MaxConnectionsPerHost = v
		}
	case "max_global_connections":
		if v, err := strconv.Atoi(value); err == nil {
			m.Settings.Connections.MaxGlobalConnections = v
		}
	case "user_agent":
		m.Settings.Connections.UserAgent = value
	}
	return nil
}

func (m *RootModel) setChunksSetting(key, value, typ string) error {
	switch key {
	case "min_chunk_size":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			m.Settings.Chunks.MinChunkSize = v
		}
	case "max_chunk_size":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			m.Settings.Chunks.MaxChunkSize = v
		}
	case "target_chunk_size":
		if v, err := strconv.ParseInt(value, 10, 64); err == nil {
			m.Settings.Chunks.TargetChunkSize = v
		}
	case "worker_buffer_size":
		if v, err := strconv.Atoi(value); err == nil {
			m.Settings.Chunks.WorkerBufferSize = v
		}
	}
	return nil
}

func (m *RootModel) setPerformanceSetting(key, value, typ string) error {
	switch key {
	case "max_task_retries":
		if v, err := strconv.Atoi(value); err == nil {
			m.Settings.Performance.MaxTaskRetries = v
		}
	case "slow_worker_threshold":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			m.Settings.Performance.SlowWorkerThreshold = v
		}
	case "slow_worker_grace_period":
		if v, err := time.ParseDuration(value); err == nil {
			m.Settings.Performance.SlowWorkerGracePeriod = v
		}
	case "stall_timeout":
		if v, err := time.ParseDuration(value); err == nil {
			m.Settings.Performance.StallTimeout = v
		}
	case "speed_ema_alpha":
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			m.Settings.Performance.SpeedEmaAlpha = v
		}
	}
	return nil
}

// getCurrentSettingKey returns the key of the currently selected setting
func (m RootModel) getCurrentSettingKey() string {
	categories := config.CategoryOrder()
	metadata := config.GetSettingsMetadata()
	currentCategory := categories[m.SettingsActiveTab]
	settingsMeta := metadata[currentCategory]

	if m.SettingsSelectedRow < len(settingsMeta) {
		return settingsMeta[m.SettingsSelectedRow].Key
	}
	return ""
}

// getCurrentSettingType returns the type of the currently selected setting
func (m RootModel) getCurrentSettingType() string {
	categories := config.CategoryOrder()
	metadata := config.GetSettingsMetadata()
	currentCategory := categories[m.SettingsActiveTab]
	settingsMeta := metadata[currentCategory]

	if m.SettingsSelectedRow < len(settingsMeta) {
		return settingsMeta[m.SettingsSelectedRow].Type
	}
	return ""
}

// getSettingsCount returns the number of settings in the current category
func (m RootModel) getSettingsCount() int {
	categories := config.CategoryOrder()
	metadata := config.GetSettingsMetadata()
	currentCategory := categories[m.SettingsActiveTab]
	return len(metadata[currentCategory])
}

// formatSettingValue formats a setting value for display
func formatSettingValue(value interface{}, typ string) string {
	if value == nil {
		return "-"
	}

	switch typ {
	case "bool":
		if b, ok := value.(bool); ok {
			if b {
				return "True"
			}
			return "False"
		}
	case "duration":
		if d, ok := value.(time.Duration); ok {
			return d.String()
		}
	case "int64":
		if v, ok := value.(int64); ok {
			return formatBytes(v)
		}
	case "float64":
		if v, ok := value.(float64); ok {
			return fmt.Sprintf("%.2f", v)
		}
	case "string":
		if s, ok := value.(string); ok {
			if s == "" {
				return "(default)"
			}
			if len(s) > 30 {
				return s[:27] + "..."
			}
			return s
		}
	}

	// Fallback using reflection for numeric types
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Int, reflect.Int64:
		return fmt.Sprintf("%d", v.Int())
	case reflect.Float64:
		return fmt.Sprintf("%.2f", v.Float())
	default:
		return fmt.Sprintf("%v", value)
	}
}
