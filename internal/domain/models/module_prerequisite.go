package models

type ModulePrerequisite struct {
	ID                   uint `json:"id" gorm:"primaryKey"`
	ModuleID             uint `json:"module_id" gorm:"index;not null"`
	PrerequisiteModuleID uint `json:"prerequisite_module_id" gorm:"not null"`
}
