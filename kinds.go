package ownerchanger

import (
	"time"
)

type Status struct {
	Id       string `datastore:"-" goon:"id"`
	IdStr    string
	From     string
	To       string
	DriveId  string
	DriveNm  string
	MimeType string

	// upload
	Type       string // 1:manual 2:csv
	Accept     string
	Note       string
	User       string
	InsertDate time.Time
	UpdateDate time.Time
	Version    string
}

type StatusInfo struct {
	Id           string `datastore:"-" goon:"id"`
	IdStr        string
	ParnetId     string
	DriveId      string
	DriveNm      string
	MimeType     string
	PermissionId string
	Result       string
	User         string
	InsertDate   time.Time
	UpdateDate   time.Time
	Version      string
}

type Suggest struct {
	Id         string `datastore:"-" goon:"id"`
	IdStr      string
	Admin      string
	Users      string
	User       string
	InsertDate time.Time
	UpdateDate time.Time
	Version    string
}
