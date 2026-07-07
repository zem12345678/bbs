package po

import "time"

type Link struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Key         string    `json:"key" gorm:"column:key;type:varchar(64);uniqueIndex;not null"`
	Title       string    `json:"title" gorm:"column:title;type:varchar(128);not null"`
	URL         string    `json:"url" gorm:"column:url;type:varchar(512);not null"`
	Description string    `json:"description" gorm:"column:description;type:varchar(512)"`
	Status      int32     `json:"status" gorm:"column:status;index;not null;default:2"`
	Sort        int32     `json:"sort" gorm:"column:sort;index;not null;default:0"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (Link) TableName() string {
	return "admin_link"
}

type Task struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Key          string    `json:"key" gorm:"column:key;type:varchar(64);uniqueIndex;not null"`
	Title        string    `json:"title" gorm:"column:title;type:varchar(128);not null"`
	Description  string    `json:"description" gorm:"column:description;type:varchar(512)"`
	RewardPoints int64     `json:"rewardPoints" gorm:"column:reward_points;not null;default:0"`
	Status       int32     `json:"status" gorm:"column:status;index;not null;default:2"`
	Sort         int32     `json:"sort" gorm:"column:sort;index;not null;default:0"`
	CreatedAt    time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt    time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (Task) TableName() string {
	return "admin_task"
}

type Badge struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Key         string    `json:"key" gorm:"column:key;type:varchar(64);uniqueIndex;not null"`
	Name        string    `json:"name" gorm:"column:name;type:varchar(128);not null"`
	Description string    `json:"description" gorm:"column:description;type:varchar(512)"`
	IconURL     string    `json:"iconUrl" gorm:"column:icon_url;type:varchar(512)"`
	RuleType    string    `json:"ruleType" gorm:"column:rule_type;type:varchar(64);not null;default:'manual'"`
	RuleValue   int64     `json:"ruleValue" gorm:"column:rule_value;not null;default:0"`
	Status      int32     `json:"status" gorm:"column:status;index;not null;default:2"`
	Sort        int32     `json:"sort" gorm:"column:sort;index;not null;default:0"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (Badge) TableName() string {
	return "admin_badge"
}

type Level struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Key         string    `json:"key" gorm:"column:key;type:varchar(64);uniqueIndex;not null"`
	Name        string    `json:"name" gorm:"column:name;type:varchar(128);not null"`
	Description string    `json:"description" gorm:"column:description;type:varchar(512)"`
	MinScore    int64     `json:"minScore" gorm:"column:min_score;index;not null;default:0"`
	MaxScore    int64     `json:"maxScore" gorm:"column:max_score;index;not null;default:0"`
	Status      int32     `json:"status" gorm:"column:status;index;not null;default:2"`
	Sort        int32     `json:"sort" gorm:"column:sort;index;not null;default:0"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (Level) TableName() string {
	return "admin_level"
}

type ForbiddenWord struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Word        string    `json:"word" gorm:"column:word;type:varchar(128);uniqueIndex:idx_forbidden_word_scene;not null"`
	Scene       string    `json:"scene" gorm:"column:scene;type:varchar(64);uniqueIndex:idx_forbidden_word_scene;not null;default:'content'"`
	Action      string    `json:"action" gorm:"column:action;type:varchar(32);not null;default:'reject'"`
	Replacement string    `json:"replacement" gorm:"column:replacement;type:varchar(128)"`
	Description string    `json:"description" gorm:"column:description;type:varchar(512)"`
	Status      int32     `json:"status" gorm:"column:status;index;not null;default:2"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (ForbiddenWord) TableName() string {
	return "admin_forbidden_word"
}

type SiteSetting struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Key         string    `json:"key" gorm:"column:key;type:varchar(128);uniqueIndex;not null"`
	Value       string    `json:"value" gorm:"column:value;type:text;not null;default:''"`
	Group       string    `json:"group" gorm:"column:setting_group;type:varchar(64);index;not null;default:'site'"`
	ValueType   string    `json:"valueType" gorm:"column:value_type;type:varchar(32);not null;default:'string'"`
	Description string    `json:"description" gorm:"column:description;type:varchar(512)"`
	Status      int32     `json:"status" gorm:"column:status;index;not null;default:2"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (SiteSetting) TableName() string {
	return "admin_site_setting"
}

type EmailLog struct {
	ID          int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	To          string    `json:"to" gorm:"column:mail_to;type:varchar(255);index;not null"`
	Subject     string    `json:"subject" gorm:"column:subject;type:varchar(255);not null"`
	TemplateKey string    `json:"templateKey" gorm:"column:template_key;type:varchar(128);index"`
	Provider    string    `json:"provider" gorm:"column:provider;type:varchar(64)"`
	Status      int32     `json:"status" gorm:"column:status;index;not null;default:1"`
	Error       string    `json:"error" gorm:"column:error;type:text"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at;index"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

func (EmailLog) TableName() string {
	return "admin_email_log"
}
