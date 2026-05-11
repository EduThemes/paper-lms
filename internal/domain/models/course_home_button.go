package models

type CourseHomeButton struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	CourseID   uint   `json:"course_id" gorm:"not null;index"`
	ButtonType string `json:"button_type" gorm:"not null"` // "todays_lesson", "continue", "my_work", "inbox", "announcements", "custom"
	Label      string `json:"label"`
	Icon       string `json:"icon"`       // Lucide icon name e.g. "Play", "BookOpen"
	Color      string `json:"color"`      // Hex color e.g. "#0374B5"
	LinkType   string `json:"link_type"`  // "auto", "page", "module", "assignment", "discussion", "external_url"
	LinkID     *uint  `json:"link_id"`    // ID of linked content (null for external URLs / presets)
	LinkURL    string `json:"link_url"`   // URL for external links
	Position   int    `json:"position"`
	Visible    bool   `json:"visible" gorm:"default:true"`
}
