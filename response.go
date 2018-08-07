package ownerchanger

type ResponseJson struct {
	Status          string
	Message         string
	Admin           string
	Users           string
	ListSize        int
	DriveDatas      []DriveData
	StatusDatas     []Status
	StatusInfoDatas []StatusInfo
	NextPageToken   string
	Share           string
	CreatedTime     string
	ModifiedTime    string
	MimeType        string
}

type DriveData struct {
	Id           string
	Name         string
	MimeType     string
	Owner        string
	OwnerNm      string
	PermissionId string
	IconLink     string
}
