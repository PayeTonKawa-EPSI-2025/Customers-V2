package models

type Order struct {
	ID uint `json:"id" gorm:"primaryKey;column:id"`
}
