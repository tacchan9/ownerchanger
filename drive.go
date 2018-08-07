package ownerchanger

import (
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"io/ioutil"
	"strconv"
	"strings"

	"net/http"
	"net/url"

	"encoding/csv"
	"encoding/json"
	"github.com/mjibson/goon"
	"github.com/zenazn/goji/web"
	"golang.org/x/net/context"
	"google.golang.org/api/admin/directory/v1"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/taskqueue"
	"google.golang.org/appengine/user"
	"math/rand"
	"time"
)

// Path to the Service Account's Private Key file
var ServiceAccountFilePath = "json/xxxxx.json"

var dataStoreVersion = "1"

var taskQueueName = "ownerchanger"

var listSize = 10

// rand start
var rs1Letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandString1(n int) string {
	rand.Seed(time.Now().UnixNano())
	b := make([]rune, n)
	for i := range b {
		b[i] = rs1Letters[rand.Intn(len(rs1Letters))]
	}
	return string(b)
}

// rand finish

func getClient(userEmail string, rq *http.Request) *http.Client {

	c := appengine.NewContext(rq)

	// timeout
	c, _ = context.WithTimeout(c, 10*time.Second)

	jsonCredentials, err := ioutil.ReadFile(ServiceAccountFilePath)
	if err != nil {
		log.Errorf(c, "error reading credentials from file: %v", err)
		panic(fmt.Sprintf("error reading credentials from file: %v", err))
	}

	config, err := google.JWTConfigFromJSON(jsonCredentials, drive.DriveScope)
	if err != nil {
		log.Errorf(c, "Unable to parse client secret file to config: %v", err)
		panic(fmt.Sprintf("Unable to parse client secret file to config: %v", err))
	}

	// execute account
	config.Subject = userEmail

	return config.Client(c)
}

func getAdminClient(userEmail string, rq *http.Request) *http.Client {

	c := appengine.NewContext(rq)

	// timeout
	c, _ = context.WithTimeout(c, 10*time.Second)

	jsonCredentials, err := ioutil.ReadFile(ServiceAccountFilePath)
	if err != nil {
		log.Errorf(c, "error reading credentials from file: %v", err)
		panic(fmt.Sprintf("error reading credentials from file: %v", err))
	}

	config, err := google.JWTConfigFromJSON(jsonCredentials, admin.AdminDirectoryUserReadonlyScope)
	if err != nil {
		log.Errorf(c, "Unable to parse client secret file to config: %v", err)
		panic(fmt.Sprintf("Unable to parse client secret file to config: %v", err))
	}

	// execute account
	config.Subject = userEmail

	return config.Client(c)
}

func userCheck(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "userCheck"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	userEmail := rq.FormValue("userEmail")
	log.Infof(ctx, "userEmail: %s\n", userEmail)

	client := getClient(userEmail, rq)

	srv, err := drive.New(client)
	if err != nil {
		log.Errorf(ctx, "%s :drive.New error %v", funcNm, err)
		panic(fmt.Sprintf("%s :drive.New error %v", funcNm, err))
	}

	_, err = srv.Files.List().PageSize(1).Fields("nextPageToken, files(id,mimeType,name,owners)").Do()
	if err != nil {
		log.Infof(ctx, "%s :srv.Files.List error %v", funcNm, err)
		// panic(fmt.Sprintf("%s :srv.Files.List error %v", funcNm, err))
		rj := ResponseJson{Status: "ng", Message: "srv.Files.List error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	rj := ResponseJson{Status: "ok"}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * @urlParam userEmail
 * @urlParam driveId
 * @urlParam nextPageToken
 */
func driveList(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "driveList"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	userEmail := rq.FormValue("userEmail")

	driveId := rq.FormValue("driveId")
	nextPageToken := rq.FormValue("nextPageToken")
	searchTxt := rq.FormValue("searchTxt")

	client := getClient(userEmail, rq)

	srv, err := drive.New(client)
	if err != nil {
		log.Errorf(ctx, "%s :drive.New error %v", funcNm, err)
		panic(fmt.Sprintf("%s :drive.New error %v", funcNm, err))
	}

	// search
	q := "'" + driveId + "' in parents and trashed = false"

	if searchTxt != "" {
		//fullText contains 'hello'
		q = "fullText contains '" + searchTxt + "' and trashed = false"
	}

	log.Infof(ctx, "%s", q)
	r, err := srv.Files.List().PageSize(10).Q(q).PageToken(nextPageToken).Fields("nextPageToken, files(id,mimeType,name,owners,iconLink)").Do()
	if err != nil {
		log.Errorf(ctx, "%s :srv.Files.List error %v", funcNm, err)
		// panic(fmt.Sprintf("%s :srv.Files.List error %v", funcNm, err))
		rj := ResponseJson{Status: "ng", Message: "srv.Files.List error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	log.Infof(ctx, "Files: %d", len(r.Files))

	driveDatas := make([]DriveData, len(r.Files))
	nextPageToken = ""

	if len(r.Files) > 0 {

		nextPageToken = r.NextPageToken
		log.Infof(ctx, "nextPageToken: %s\n", nextPageToken)

		for i, f := range r.Files {
			log.Infof(ctx, "fileNm: %s\n", f.Name)
			log.Infof(ctx, "fileId: %s\n", f.Id)
			log.Infof(ctx, "mimeType: %s\n", f.MimeType)
			log.Infof(ctx, "iconLink: %s\n", strings.Replace(f.IconLink, "/16/", "/32/", 1))

			driveDatas[i].Id = f.Id
			driveDatas[i].Name = f.Name
			driveDatas[i].MimeType = f.MimeType
			driveDatas[i].IconLink = strings.Replace(f.IconLink, "/16/", "/32/", 1)

			for _, o := range f.Owners {
				log.Infof(ctx, "ownerNm: %s\n", o.DisplayName)
				log.Infof(ctx, "owner: %s\n", o.EmailAddress)
				log.Infof(ctx, "permissionId: %s\n", o.PermissionId)

				driveDatas[i].Owner = o.EmailAddress
				driveDatas[i].OwnerNm = o.DisplayName
				driveDatas[i].PermissionId = o.PermissionId
			}
		}
	}

	rj := ResponseJson{Status: "ok", DriveDatas: driveDatas, NextPageToken: nextPageToken, ListSize: len(r.Files)}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * @urlParam fromUser
 * @urlParam toUser
 * @urlParam driveId
 * @urlParam driveNm
 * @urlParam mimeType
 * @urlParam owner
 * @urlParam permissionId
 */
func ownerChange(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "ownerChange"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	userEmail := rq.FormValue("fromUser")
	toUser := rq.FormValue("toUser")
	mimeType := rq.FormValue("mimeType")
	owner := rq.FormValue("owner")
	driveId := rq.FormValue("driveId")
	driveNm := rq.FormValue("driveNm")
	permissionId := rq.FormValue("permissionId")

	log.Infof(ctx, "fileNm: %s\n", driveNm)

	statusId := RandString1(50)

	// datastore
	g := goon.NewGoon(rq)

	status := Status{
		Id:         statusId,
		IdStr:      statusId,
		From:       userEmail,
		To:         toUser,
		DriveId:    driveId,
		DriveNm:    driveNm,
		MimeType:   mimeType,
		Type:       "1",
		Accept:     "OK",
		User:       u.String(),
		InsertDate: time.Now(),
		UpdateDate: time.Now(),
		Version:    dataStoreVersion,
	}

	key, err := g.Put(&status)

	if err != nil {
		log.Errorf(ctx, "%s :g.Put %v", funcNm, err)
		panic(fmt.Sprintf("%s :g.Put %v", funcNm, err))
	}
	log.Infof(ctx, "key: %s\n", key.String())

	// urlParam(cron)
	urlParams := url.Values{}
	urlParams.Set("fromUser", userEmail)
	urlParams.Set("toUser", toUser)
	urlParams.Set("statusId", statusId)
	urlParams.Set("execUser", u.String())
	urlParams.Set("mimeType", mimeType)
	urlParams.Set("owner", owner)
	urlParams.Set("driveId", driveId)
	urlParams.Set("driveNm", driveNm)
	urlParams.Set("permissionId", permissionId)

	// taskqueue
	task := taskqueue.NewPOSTTask("/ownerChangeCron", urlParams)
	_, err = taskqueue.Add(ctx, task, taskQueueName)
	if err != nil {
		log.Errorf(ctx, "%s :taskqueue.Add error %v", funcNm, err)
		panic(fmt.Sprintf("%s :taskqueue.Add error %v", funcNm, err))
	}

	rj := ResponseJson{Status: "ok"}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * @urlParam fromUser
 * @urlParam toUser
 * @urlParam driveId
 * @urlParam driveNm
 * @urlParam mimeType
 * @urlParam owner
 * @urlParam permissionId
 * @urlParam nextPageToken
 * @urlParam statusId
 * @urlParam execUser
 */
func ownerChangeCron(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "ownerChangeCron"

	ctx := appengine.NewContext(rq)

	u := rq.FormValue("execUser")

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	userEmail := rq.FormValue("fromUser")
	toUser := rq.FormValue("toUser")
	mimeType := rq.FormValue("mimeType")
	owner := rq.FormValue("owner")
	driveId := rq.FormValue("driveId")
	driveNm := rq.FormValue("driveNm")
	permissionId := rq.FormValue("permissionId")
	statusId := rq.FormValue("statusId")
	nextPageToken := rq.FormValue("nextPageToken")

	log.Infof(ctx, "fileNm: %s\n", driveNm)

	log.Infof(ctx, "userEmail: %s\n", userEmail)
	log.Infof(ctx, "toUser: %s\n", toUser)
	log.Infof(ctx, "mimeType: %s\n", mimeType)
	log.Infof(ctx, "owner: %s\n", owner)
	log.Infof(ctx, "driveId: %s\n", driveId)
	log.Infof(ctx, "permissionId: %s\n", permissionId)
	log.Infof(ctx, "statusId: %s\n", statusId)
	log.Infof(ctx, "nextPageToken: %s\n", nextPageToken)

	client := getClient(userEmail, rq)

	srv, err := drive.New(client)
	if err != nil {
		log.Errorf(ctx, "%s :drive.New error %v", funcNm, err)
		ownerChangeCronError(c, w, rq)
		panic(fmt.Sprintf("%s :drive.New error %v", funcNm, err))
	}

	// folder
	if mimeType == "application/vnd.google-apps.folder" {
		// search
		q := "'" + driveId + "' in parents"

		log.Infof(ctx, "%s", q)
		log.Infof(ctx, "%s", nextPageToken)
		r, err := srv.Files.List().PageSize(10).Q(q).PageToken(nextPageToken).Fields("nextPageToken, files(id,mimeType,name,owners)").Do()
		if err != nil {
			log.Errorf(ctx, "%s :srv.Files.List error %v", funcNm, err)
			ownerChangeCronError(c, w, rq)
			panic(fmt.Sprintf("%s :srv.Files.List error %v", funcNm, err))
		}

		log.Infof(ctx, "Files: %d", len(r.Files))

		nextPageToken = r.NextPageToken
		log.Infof(ctx, "nextPageToken: %s\n", nextPageToken)

		// folder continue
		if nextPageToken != "" {

			// urlParam(cron)
			urlParams := url.Values{}
			urlParams.Set("fromUser", userEmail)
			urlParams.Set("toUser", toUser)
			urlParams.Set("statusId", statusId)
			urlParams.Set("execUser", u)
			urlParams.Set("mimeType", mimeType)
			urlParams.Set("owner", owner)
			urlParams.Set("driveId", driveId)
			urlParams.Set("driveNm", driveNm)
			urlParams.Set("permissionId", permissionId)
			urlParams.Set("nextPageToken", nextPageToken)

			// taskqueue
			task := taskqueue.NewPOSTTask("/ownerChangeCron", urlParams)
			_, err := taskqueue.Add(ctx, task, taskQueueName)
			if err != nil {
				log.Errorf(ctx, "%s :taskqueue.Add error %v", funcNm, err)
				ownerChangeCronError(c, w, rq)
				panic(fmt.Sprintf("%s :taskqueue.Add error %v", funcNm, err))

			}
		}

		if len(r.Files) > 0 {

			for _, f := range r.Files {

				for _, o := range f.Owners {

					// urlParam(cron)
					urlParams := url.Values{}
					urlParams.Set("fromUser", userEmail)
					urlParams.Set("toUser", toUser)
					urlParams.Set("statusId", statusId)
					urlParams.Set("execUser", u)

					// folder or owner
					if f.MimeType == "application/vnd.google-apps.folder" || o.EmailAddress == userEmail {

						urlParams.Set("mimeType", f.MimeType)
						urlParams.Set("owner", o.EmailAddress)
						urlParams.Set("driveId", f.Id)
						urlParams.Set("driveNm", f.Name)
						urlParams.Set("permissionId", o.PermissionId)
						urlParams.Set("nextPageToken", "")

						// taskqueue
						task := taskqueue.NewPOSTTask("/ownerChangeCron", urlParams)
						_, err := taskqueue.Add(ctx, task, taskQueueName)
						if err != nil {
							log.Errorf(ctx, "%s :taskqueue.Add error %v", funcNm, err)
							ownerChangeCronError(c, w, rq)
							panic(fmt.Sprintf("%s :taskqueue.Add error %v", funcNm, err))

						}

					}
				}
			}
		}
	}

	// ownerchange
	if userEmail == owner && rq.FormValue("nextPageToken") == "" {

		// create writer
		p := &drive.Permission{Role: "writer", Type: "user", EmailAddress: toUser}
		cr, err := srv.Permissions.Create(driveId, p).SendNotificationEmail(false).Do()
		if err != nil {
			log.Errorf(ctx, "%s :srv.Permissions.Create %v", funcNm, err)
			ownerChangeCronError(c, w, rq)
			panic(fmt.Sprintf("%s :srv.Permissions.Create %v", funcNm, err))
		}

		r := &drive.Permission{Role: "owner"}
		_, err = srv.Permissions.Update(driveId, cr.Id, r).TransferOwnership(true).Do()
		if err != nil {
			log.Errorf(ctx, "%s :srv.Permissions.Update %v", funcNm, err)
			ownerChangeCronError(c, w, rq)
			panic(fmt.Sprintf("%s :srv.Permissions.Update %v", funcNm, err))
		}

		// datastore
		g := goon.NewGoon(rq)

		id := RandString1(50)

		statusInfo := StatusInfo{
			Id:           id,
			IdStr:        id,
			ParnetId:     statusId,
			DriveId:      driveId,
			DriveNm:      driveNm,
			MimeType:     mimeType,
			PermissionId: permissionId,
			Result:       "OK",
			User:         u,
			InsertDate:   time.Now(),
			UpdateDate:   time.Now(),
			Version:      dataStoreVersion,
		}

		//retry
		q := datastore.NewQuery("StatusInfo").Filter("ParnetId =", statusId).Filter("DriveId =", driveId)

		ngStatusInfoLists := make([]StatusInfo, 0)
		g.GetAll(q, &ngStatusInfoLists)

		if len(ngStatusInfoLists) != 0 {
			log.Infof(ctx, "ng update")

			statusInfo = StatusInfo{
				Id:           ngStatusInfoLists[0].Id,
				IdStr:        ngStatusInfoLists[0].IdStr,
				ParnetId:     statusId,
				DriveId:      driveId,
				DriveNm:      driveNm,
				MimeType:     mimeType,
				PermissionId: permissionId,
				Result:       "OK",
				User:         u,
				InsertDate:   ngStatusInfoLists[0].InsertDate,
				UpdateDate:   time.Now(),
				Version:      dataStoreVersion,
			}
		}

		if _, err := g.Put(&statusInfo); err != nil {
			log.Errorf(ctx, "%s :g.Put %v", funcNm, err)
			panic(fmt.Sprintf("%s :g.Put %v", funcNm, err))
		}
	}

	rj := ResponseJson{Status: "ok"}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

func ownerChangeCronError(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "ownerChangeCronError"

	ctx := appengine.NewContext(rq)

	u := rq.FormValue("execUser")

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	//toUser := rq.FormValue("toUser")
	mimeType := rq.FormValue("mimeType")
	driveId := rq.FormValue("driveId")
	driveNm := rq.FormValue("driveNm")
	permissionId := rq.FormValue("permissionId")
	statusId := rq.FormValue("statusId")

	log.Infof(ctx, "fileNm: %s\n", driveNm)

	// datastore
	g := goon.NewGoon(rq)

	status := &Status{Id: statusId}

	if err := g.Get(status); err != nil {
		log.Errorf(ctx, "%s :g.Get %v", funcNm, err)
		panic(fmt.Sprintf("%s :g.Get %v", funcNm, err))
	}

	ngStatus := Status{
		Id:         status.Id,
		IdStr:      status.IdStr,
		From:       status.From,
		To:         status.To,
		DriveId:    status.DriveId,
		DriveNm:    status.DriveNm,
		MimeType:   status.MimeType,
		Type:       status.Type,
		Accept:     "CK",
		Note:       status.Note,
		User:       u,
		InsertDate: status.InsertDate,
		UpdateDate: time.Now(),
		Version:    status.Version,
	}

	if _, err := g.Put(&ngStatus); err != nil {
		log.Errorf(ctx, "%s :g.Put %v", funcNm, err)
		panic(fmt.Sprintf("%s :g.Put %v", funcNm, err))
	}

	id := RandString1(50)

	statusInfo := StatusInfo{
		Id:           id,
		IdStr:        id,
		ParnetId:     statusId,
		DriveId:      driveId,
		DriveNm:      driveNm,
		MimeType:     mimeType,
		PermissionId: permissionId,
		Result:       "NG",
		User:         u,
		InsertDate:   time.Now(),
		UpdateDate:   time.Now(),
		Version:      dataStoreVersion,
	}

	//retry
	q := datastore.NewQuery("StatusInfo").Filter("ParnetId =", statusId).Filter("DriveId =", driveId)

	ngStatusInfoLists := make([]StatusInfo, 0)
	_, err := g.GetAll(q, &ngStatusInfoLists)
	if err != nil {
		log.Infof(ctx, "%s :g.GetAll %v", funcNm, err)
	}

	if len(ngStatusInfoLists) != 0 {
		log.Infof(ctx, "ng update")

		statusInfo = StatusInfo{
			Id:           ngStatusInfoLists[0].Id,
			IdStr:        ngStatusInfoLists[0].IdStr,
			ParnetId:     statusId,
			DriveId:      driveId,
			DriveNm:      driveNm,
			MimeType:     mimeType,
			PermissionId: permissionId,
			Result:       "NG",
			User:         u,
			InsertDate:   ngStatusInfoLists[0].InsertDate,
			UpdateDate:   time.Now(),
			Version:      dataStoreVersion,
		}
	}

	if _, err := g.Put(&statusInfo); err != nil {
		log.Errorf(ctx, "%s :g.Put %v", funcNm, err)
		panic(fmt.Sprintf("%s :g.Put %v", funcNm, err))
	}

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)
	panic(fmt.Sprintf("%s :error", funcNm))

}

/*
 * not use
 *  didn't know hot to use nextToken
 */
func statusList(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "statusList"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	g := goon.NewGoon(rq)

	// query
	q := datastore.NewQuery("Status").Order("-InsertDate")

	//q := datastore.NewQuery("Person").Order("-Height").Limit(5).Offset(5)

	statusLists := make([]Status, 0)

	g.GetAll(q, &statusLists)

	rj := ResponseJson{Status: "ok", StatusDatas: statusLists, NextPageToken: "", ListSize: len(statusLists)}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * not use
 *  didn't know hot to use nextToken
 */
func statusInfoList(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "statusInfoList"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	statusId := rq.FormValue("statusId")

	g := goon.NewGoon(rq)

	// query
	q := datastore.NewQuery("StatusInfo").Filter("ParnetId =", statusId)

	statusInfoLists := make([]StatusInfo, 0)

	g.GetAll(q, &statusInfoLists)

	rj := ResponseJson{Status: "ok", StatusInfoDatas: statusInfoLists, NextPageToken: "", ListSize: len(statusInfoLists)}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * @urlParam nextPageToken
 * @urlParam searchTxt
 */
func statusListCursor(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "statusListCursor"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	searchTxt := rq.FormValue("searchTxt")

	// query
	q := datastore.NewQuery("Status").Order("-InsertDate").Limit(listSize)

	if searchTxt != "" {

		log.Infof(ctx, "searchTxt: %s", searchTxt)

		// Date
		if strings.Index(searchTxt, "Date") == 0 {
			//strdate := "2018-02-10 18:04:35 +0900 JST"
			str := strings.Split(searchTxt, ":")
			//strdate := str[1] + " 00:00:00 +0900 JST"
			strdate := str[1] + " 23:59:59 +0900 JST"
			log.Infof(ctx, "searchTxt: %s", strdate)
			inputtime, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", strdate)
			q = datastore.NewQuery("Status").Filter("InsertDate <=", inputtime).Order("-InsertDate").Limit(listSize)

			// Name
		} else if strings.Index(searchTxt, "Name") == 0 {
			str := strings.Split(strings.Replace(searchTxt, "Name:", "DriveNm:", 1), ":")

			//prefix search
			// (datastore_v3: BAD_REQUEST): inequality filter property and first sort order must be the same: DriveNm and InsertDate
			q = datastore.NewQuery("Status").Filter(str[0]+">=", str[1]).Filter(str[0]+"<", str[1]+"\ufffd").Order("DriveNm").Order("-InsertDate").Limit(listSize)

			// CsvName
		} else if strings.Index(searchTxt, "CsvName") == 0 {
			str := strings.Split(strings.Replace(searchTxt, "CsvName:", "Note:", 1), ":")

			//prefix search
			q = datastore.NewQuery("Status").Filter(str[0]+">=", str[1]+":").Filter(str[0]+"<", str[1]+":"+"\ufffd").Order("Note").Order("-InsertDate").Limit(listSize)

			// User From To Result
		} else {
			str := strings.Split(strings.Replace(searchTxt, "Result:", "Accept:", 1), ":")
			q = datastore.NewQuery("Status").Filter(str[0]+"=", str[1]).Order("-InsertDate").Limit(listSize)

		}

	}

	cursor, err := datastore.DecodeCursor(rq.FormValue("nextPageToken"))
	if err == nil {
		q = q.Start(cursor)
	}

	count, err := q.Count(ctx)
	if err != nil {
		log.Errorf(ctx, "%s :q.Count %v", funcNm, err)
		panic(fmt.Sprintf("%s :q.Count %v", funcNm, err))
	}
	log.Infof(ctx, "count: %d", count)

	t := q.Run(ctx)

	statusLists := make([]Status, count)
	idx := 0

	for {
		var status Status
		_, err := t.Next(&status)
		if err == datastore.Done {
			break
		}
		if err != nil {
			log.Errorf(ctx, "%s :q.t.Next(&status) %v", funcNm, err)
			break
		}

		log.Infof(ctx, "%v", status)
		statusLists[idx].Id = status.IdStr
		statusLists[idx].DriveNm = status.DriveNm
		statusLists[idx].From = status.From
		statusLists[idx].To = status.To
		statusLists[idx].MimeType = status.MimeType
		statusLists[idx].Accept = status.Accept
		statusLists[idx].Note = status.Note
		statusLists[idx].User = status.User

		jst := time.FixedZone("Asia/Tokyo", 9*60*60)
		ins := status.InsertDate.In(jst)
		//statusLists[idx].InsertDate = ins.Format(time.RFC3339)
		statusLists[idx].InsertDate = ins
		idx++
	}

	cursor, _ = t.Cursor()

	rj := ResponseJson{Status: "ok", StatusDatas: statusLists, NextPageToken: cursor.String(), ListSize: len(statusLists)}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * @urlParam statusId
 * @urlParam nextPageToken
 * @urlParam searchTxt
 */
func statusInfoListCursor(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "statusInfoListCursor"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	statusId := rq.FormValue("statusId")

	searchTxt := rq.FormValue("searchTxt")

	// query
	q := datastore.NewQuery("StatusInfo").Filter("ParnetId =", statusId).Order("-InsertDate").Limit(listSize)

	if searchTxt != "" {

		log.Infof(ctx, "searchTxt: %s", searchTxt)

		// Date
		if strings.Index(searchTxt, "Date") == 0 {
			//strdate := "2018-02-10 18:04:35 +0900 JST"
			str := strings.Split(searchTxt, ":")
			strdate := str[1] + " 00:00:00 +0900 JST"
			log.Infof(ctx, "searchTxt: %s", strdate)
			inputtime, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", strdate)
			q = datastore.NewQuery("StatusInfo").Filter("ParnetId =", statusId).Filter("InsertDate <=", inputtime).Limit(listSize)

		} else {
			str := strings.Split(strings.Replace(searchTxt, "Name:", "DriveNm:", 1), ":")
			//q = datastore.NewQuery("StatusInfo").Filter("ParnetId =", statusId).Filter(str[0]+"=", str[1]).Limit(listSize)

			//prefix search
			// .Filter("Hoge>=", hoge).Filter("Hoge<", hoge+"\ufffd")
			q = datastore.NewQuery("StatusInfo").Filter("ParnetId =", statusId).Filter(str[0]+">=", str[1]).Filter(str[0]+"<", str[1]+"\ufffd").Limit(listSize)

		}

	}

	cursor, err := datastore.DecodeCursor(rq.FormValue("nextPageToken"))
	if err == nil {
		q = q.Start(cursor)
	}

	count, err := q.Count(ctx)

	if err != nil {
		log.Errorf(ctx, "%s :q.Count %v", funcNm, err)
		panic(fmt.Sprintf("%s :q.Count %v", funcNm, err))
	}

	log.Infof(ctx, "count: %d", count)

	t := q.Run(ctx)

	statusInfoLists := make([]StatusInfo, count)
	idx := 0

	for {
		var statusInfo StatusInfo
		_, err := t.Next(&statusInfo)
		if err == datastore.Done {
			break
		}
		if err != nil {
			log.Errorf(ctx, "%s :q.t.Next(&status) %v", funcNm, err)
			break
		}

		log.Infof(ctx, "%v", statusInfo)
		statusInfoLists[idx].DriveNm = statusInfo.DriveNm
		statusInfoLists[idx].MimeType = statusInfo.MimeType
		statusInfoLists[idx].Result = statusInfo.Result

		jst := time.FixedZone("Asia/Tokyo", 9*60*60)
		ins := statusInfo.InsertDate.In(jst)
		//statusLists[idx].InsertDate = ins.Format(time.RFC3339)
		statusInfoLists[idx].InsertDate = ins
		idx++
	}

	cursor, _ = t.Cursor()

	rj := ResponseJson{Status: "ok", StatusInfoDatas: statusInfoLists, NextPageToken: cursor.String(), ListSize: len(statusInfoLists)}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

func taskQueueInfo(c web.C, w http.ResponseWriter, rq *http.Request) {
	funcNm := "taskQueueInfo"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	taskNames := []string{taskQueueName}

	qs, err := taskqueue.QueueStats(ctx, taskNames)
	if err != nil {
		log.Errorf(ctx, "%s :taskqueue.QueueStats error %v", funcNm, err)
		panic(fmt.Sprintf("%s :taskqueue.QueueStats error %v", funcNm, err))
	}

	cnt := 0

	for _, t := range qs {
		cnt = t.Tasks
		log.Infof(ctx, "%s : cnt: %d", funcNm, t.Tasks)
	}

	rj := ResponseJson{Status: "ok", ListSize: cnt}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

func taskQueuePurge(c web.C, w http.ResponseWriter, rq *http.Request) {
	funcNm := "taskQueuePurge"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	err := taskqueue.Purge(ctx, taskQueueName)
	if err != nil {
		log.Errorf(ctx, "%s :taskqueue.Purge error %v", funcNm, err)
		panic(fmt.Sprintf("%s :taskqueue.Purge error %v", funcNm, err))
	}

	rj := ResponseJson{Status: "ok"}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

func upload(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "upload"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	f, handler, err := rq.FormFile("csv")

	// fileName
	log.Infof(ctx, "fileNm: %s\n", handler.Filename)

	if err != nil {
		log.Infof(ctx, "%s :rq.FormFile %v", funcNm, err)
		rj := ResponseJson{Status: "ng", Message: "rq.FormFile error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	defer f.Close()
	r := csv.NewReader(f)

	r.Comma = ','

	r.Comment = '#'

	records, err := r.ReadAll()
	if err != nil {

		log.Infof(ctx, "%s :r.ReadAll %v", funcNm, err)
		rj := ResponseJson{Status: "ng", Message: "r.ReadAll error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	for i, v := range records {

		// header
		if i == 0 {
			continue
		}

		// format
		if len(v) != 4 {
			log.Infof(ctx, "%s :format error %v", funcNm, err)
			rj := ResponseJson{Status: "ng", Message: "format error"}
			json.NewEncoder(w).Encode(rj)

			return
		}

		log.Infof(ctx, "col1: %s\n", v[0])
		log.Infof(ctx, "col2: %s\n", v[1])
		log.Infof(ctx, "col3: %s\n", v[2])
		log.Infof(ctx, "col4: %s\n", v[3])

		// urlParam(cron)
		urlParams := url.Values{}
		urlParams.Set("fromUser", v[0])
		urlParams.Set("toUser", v[1])
		urlParams.Set("driveId", v[2])
		urlParams.Set("driveNm", v[3])
		line := i + 1
		urlParams.Set("note", handler.Filename+":"+strconv.Itoa(line))
		urlParams.Set("execUser", u.String())

		// taskqueue
		task := taskqueue.NewPOSTTask("/ownerChangeUploadCron", urlParams)
		_, err = taskqueue.Add(ctx, task, taskQueueName)
		if err != nil {
			log.Errorf(ctx, "%s :taskqueue.Add error %v", funcNm, err)
			panic(fmt.Sprintf("%s :taskqueue.Add error %v", funcNm, err))
		}

	}

	rj := ResponseJson{Status: "ok"}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * @urlParam fromUser
 * @urlParam toUser
 * @urlParam driveId
 * @urlParam driveNm
 * @urlParam note
 * @urlParam execUser
 */
func ownerChangeUploadCron(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "ownerChangeUploadCron"

	ctx := appengine.NewContext(rq)
	u := rq.FormValue("execUser")

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	userEmail := rq.FormValue("fromUser")
	toUser := rq.FormValue("toUser")
	driveId := rq.FormValue("driveId")
	driveNm := rq.FormValue("driveNm")

	log.Infof(ctx, "fileNm: %s\n", driveNm)

	statusId := RandString1(50)

	// datastore
	g := goon.NewGoon(rq)

	status := Status{
		Id:      statusId,
		IdStr:   statusId,
		From:    userEmail,
		To:      toUser,
		DriveId: driveId,
		DriveNm: driveNm,
		//MimeType:   mimeType,
		MimeType:   "application/vnd.google-apps.folder",
		Type:       "2",
		Accept:     "NG",
		Note:       rq.FormValue("note"),
		User:       u,
		InsertDate: time.Now(),
		UpdateDate: time.Now(),
		Version:    dataStoreVersion,
	}

	key, err := g.Put(&status)

	if err != nil {
		log.Errorf(ctx, "%s :g.Put %v", funcNm, err)
		panic(fmt.Sprintf("%s :g.Put %v", funcNm, err))
	}
	log.Infof(ctx, "key: %s\n", key.String())

	// check id

	// fromUser driveId check
	client := getClient(userEmail, rq)

	srv, err := drive.New(client)
	if err != nil {
		log.Infof(ctx, "%s :from drive.New error %v", funcNm, err)
		rj := ResponseJson{Status: "ng", Message: "from drive.New error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	f, err := srv.Files.Get(driveId).Fields("id,mimeType,name,owners").Do()
	if err != nil {
		log.Errorf(ctx, "%s :from srv.Files.Get error %v", funcNm, err)
		rj := ResponseJson{Status: "ng", Message: "from srv.Files.Get error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	mimeType := ""
	owner := ""
	permissionId := ""

	log.Infof(ctx, "fileNm: %s\n", f.Name)
	log.Infof(ctx, "fileId: %s\n", f.Id)
	log.Infof(ctx, "mimeType: %s\n", f.MimeType)

	driveNm = f.Name
	mimeType = f.MimeType

	for _, o := range f.Owners {
		log.Infof(ctx, "ownerNm: %s\n", o.DisplayName)
		log.Infof(ctx, "owner: %s\n", o.EmailAddress)
		log.Infof(ctx, "permissionId: %s\n", o.PermissionId)

		owner = o.EmailAddress
		permissionId = o.PermissionId
	}

	// toUser check
	client = getClient(toUser, rq)

	srv, err = drive.New(client)
	if err != nil {
		log.Infof(ctx, "%s :to drive.New error %v", funcNm, err)
		rj := ResponseJson{Status: "ng", Message: "to drive.New error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	// search
	q := "'root' in parents and trashed = false"

	log.Infof(ctx, "%s", q)
	_, err = srv.Files.List().PageSize(1).Q(q).Fields("files(id,mimeType,name,owners)").Do()
	if err != nil {
		log.Infof(ctx, "%s :to srv.Files.List error %v", funcNm, err)
		rj := ResponseJson{Status: "ng", Message: "to srv.Files.List error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	// urlParam(cron)
	urlParams := url.Values{}
	urlParams.Set("fromUser", userEmail)
	urlParams.Set("toUser", toUser)
	urlParams.Set("statusId", statusId)
	urlParams.Set("execUser", u)
	urlParams.Set("mimeType", mimeType)
	urlParams.Set("owner", owner)
	urlParams.Set("driveId", driveId)
	urlParams.Set("driveNm", driveNm)
	urlParams.Set("permissionId", permissionId)

	// taskqueue
	task := taskqueue.NewPOSTTask("/ownerChangeCron", urlParams)
	_, err = taskqueue.Add(ctx, task, taskQueueName)
	if err != nil {
		log.Errorf(ctx, "%s :taskqueue.Add error %v", funcNm, err)
		panic(fmt.Sprintf("%s :taskqueue.Add error %v", funcNm, err))
	}

	// update
	status = Status{
		Id:         statusId,
		IdStr:      statusId,
		From:       userEmail,
		To:         toUser,
		DriveId:    driveId,
		DriveNm:    driveNm,
		MimeType:   mimeType,
		Type:       "2",
		Accept:     "OK",
		Note:       rq.FormValue("note"),
		User:       u,
		InsertDate: time.Now(), // special
		UpdateDate: time.Now(),
		Version:    dataStoreVersion,
	}

	_, err = g.Put(&status)

	if err != nil {
		log.Errorf(ctx, "%s :g.Put %v", funcNm, err)
		panic(fmt.Sprintf("%s :g.Put %v", funcNm, err))
	}

	rj := ResponseJson{Status: "ok"}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * @urlParam nextPageToken
 */
func statusInfoNgListCursor(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "statusInfoNgListCursor"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	// query
	q := datastore.NewQuery("StatusInfo").Filter("Result =", "NG").Order("-InsertDate").Limit(listSize)

	cursor, err := datastore.DecodeCursor(rq.FormValue("nextPageToken"))
	if err == nil {
		q = q.Start(cursor)
	}

	count, err := q.Count(ctx)

	if err != nil {
		log.Errorf(ctx, "%s :q.Count %v", funcNm, err)
		panic(fmt.Sprintf("%s :q.Count %v", funcNm, err))
	}

	log.Infof(ctx, "count: %d", count)

	t := q.Run(ctx)

	statusInfoLists := make([]StatusInfo, count)
	idx := 0

	for {
		var statusInfo StatusInfo
		_, err := t.Next(&statusInfo)
		if err == datastore.Done {
			break
		}
		if err != nil {
			log.Errorf(ctx, "%s :q.t.Next(&status) %v", funcNm, err)
			break
		}

		log.Infof(ctx, "%v", statusInfo)
		statusInfoLists[idx].DriveNm = statusInfo.DriveNm
		statusInfoLists[idx].MimeType = statusInfo.MimeType
		statusInfoLists[idx].Result = statusInfo.Result
		statusInfoLists[idx].ParnetId = statusInfo.ParnetId

		jst := time.FixedZone("Asia/Tokyo", 9*60*60)
		ins := statusInfo.InsertDate.In(jst)
		//statusLists[idx].InsertDate = ins.Format(time.RFC3339)
		statusInfoLists[idx].InsertDate = ins
		idx++
	}

	cursor, _ = t.Cursor()

	rj := ResponseJson{Status: "ok", StatusInfoDatas: statusInfoLists, NextPageToken: cursor.String(), ListSize: len(statusInfoLists)}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * @urlParam IdStr
 */
func getNgStatus(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "getNgStatus"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	g := goon.NewGoon(rq)

	// query
	q := datastore.NewQuery("Status").Filter("IdStr =", rq.FormValue("IdStr"))

	statusLists := make([]Status, 0)

	g.GetAll(q, &statusLists)

	rj := ResponseJson{Status: "ok", StatusDatas: statusLists, NextPageToken: "", ListSize: len(statusLists)}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

func setSuggest(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "suggest"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	adminEmail := rq.FormValue("adminEmail")

	// delete
	if adminEmail == "" {
		g := goon.NewGoon(rq)
		key := datastore.NewKey(ctx, "Suggest", "suggest", 0, nil)
		g.Delete(key)
		rj := ResponseJson{Status: "ok"}
		json.NewEncoder(w).Encode(rj)
		log.Infof(ctx, "finish %s user: %s\n", funcNm, u)
		return
	}

	client := getAdminClient(adminEmail, rq)

	srv, err := admin.New(client)
	if err != nil {
		log.Errorf(ctx, "%s :admin.New error %v", funcNm, err)
		panic(fmt.Sprintf("%s :admin.New error %v", funcNm, err))
	}

	users := ""
	nextPageToken := ""

	for {
		r, err := srv.Users.List().Customer("my_customer").MaxResults(500).PageToken(nextPageToken).Fields("nextPageToken, users(primaryEmail)").OrderBy("email").Do()
		if err != nil {
			log.Errorf(ctx, "%s :srv.Users.List error %v", funcNm, err)
			panic(fmt.Sprintf("%s :srv.Users.List error %v", funcNm, err))
		}

		log.Infof(ctx, "user: %d\n", len(r.Users))

		if len(r.Users) != 0 {
			for _, u := range r.Users {
				log.Infof(ctx, "user: %s\n", u.PrimaryEmail)
				users += u.PrimaryEmail + ","
			}
		}

		log.Infof(ctx, "nextPageToken: %s\n", r.NextPageToken)

		if r.NextPageToken == "" {
			break
		}

		nextPageToken = r.NextPageToken

	}

	// datastore
	g := goon.NewGoon(rq)

	suggest := Suggest{
		Id:         "suggest",
		IdStr:      "suggest",
		Admin:      adminEmail,
		Users:      users,
		User:       u.String(),
		InsertDate: time.Now(),
		UpdateDate: time.Now(),
		Version:    dataStoreVersion,
	}

	_, err = g.Put(&suggest)

	if err != nil {
		log.Errorf(ctx, "%s :g.Put %v", funcNm, err)
		panic(fmt.Sprintf("%s :g.Put %v", funcNm, err))
	}

	rj := ResponseJson{Status: "ok"}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

func getSuggest(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "getSuggest"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	// datastore
	g := goon.NewGoon(rq)

	suggest := &Suggest{Id: "suggest"}

	g.Get(suggest)

	rj := ResponseJson{Status: "ok", Admin: suggest.Admin, Users: suggest.Users}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

/*
 * @urlParam fromUser
 * @urlParam driveId
 * @urlParam driveNm
 * @urlParam mimeType
 */
func fileDownload(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "fileDownload"

	ctx := appengine.NewContext(rq)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	userEmail := rq.FormValue("fromUser")
	driveId := rq.FormValue("driveId")
	driveNm := rq.FormValue("driveNm")
	mimeType := rq.FormValue("mimeType")

	log.Infof(ctx, "userEmail : %s\n", userEmail)
	log.Infof(ctx, "driveId : %s\n", driveId)
	log.Infof(ctx, "driveNm : %s\n", driveNm)
	log.Infof(ctx, "mimeType : %s\n", mimeType)

	client := getClient(userEmail, rq)

	srv, err := drive.New(client)
	if err != nil {
		log.Errorf(ctx, "%s :drive.New error %v", funcNm, err)
		panic(fmt.Sprintf("%s :drive.New error %v", funcNm, err))
	}

	// google document
	if mimeType == "application/vnd.google-apps.document" {
		res, err := srv.Files.Export(driveId, "application/vnd.openxmlformats-officedocument.wordprocessingml.document").Download()
		fileNm := driveNm + ".docx"

		if err != nil {
			log.Errorf(ctx, "%s :srv.Files.Export error %v", funcNm, err)
		}
		result, _ := ioutil.ReadAll(res.Body)

		w.Header().Set("Content-Disposition", "attachment; filename="+url.QueryEscape(fileNm))
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Length", string(len(result)))

		w.Write(result)

		// google spreadsheet
	} else if mimeType == "application/vnd.google-apps.spreadsheet" {
		res, err := srv.Files.Export(driveId, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet").Download()
		fileNm := driveNm + ".xlsx"

		if err != nil {
			log.Errorf(ctx, "%s :srv.Files.Export error %v", funcNm, err)
		}
		result, _ := ioutil.ReadAll(res.Body)

		w.Header().Set("Content-Disposition", "attachment; filename="+url.QueryEscape(fileNm))
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Length", string(len(result)))

		w.Write(result)

		// google presentation
	} else if mimeType == "application/vnd.google-apps.presentation" {
		res, err := srv.Files.Export(driveId, "application/vnd.openxmlformats-officedocument.presentationml.presentation").Download()
		fileNm := driveNm + ".pptx"

		if err != nil {
			log.Errorf(ctx, "%s :srv.Files.Export error %v", funcNm, err)
		}
		result, _ := ioutil.ReadAll(res.Body)

		w.Header().Set("Content-Disposition", "attachment; filename="+url.QueryEscape(fileNm))
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Content-Length", string(len(result)))

		w.Write(result)

	} else {
		res, err := srv.Files.Get(driveId).Download()
		fileNm := driveNm

		if err != nil {
			log.Errorf(ctx, "%s :srv.Files.Export error %v", funcNm, err)

			w.Header().Set("Content-Disposition", "attachment; filename=NotSupportFomat("+mimeType+")")
			w.Header().Set("Content-Type", "text/plain")

		} else {
			result, _ := ioutil.ReadAll(res.Body)

			//w.Header().Set("Content-Type", mimeType)
			w.Header().Set("Content-Type", "application/force-download")
			w.Header().Set("Content-Length", string(len(result)))
			w.Header().Set("Content-Disposition", "attachment; filename="+url.QueryEscape(fileNm))

			w.Write(result)

		}

	}

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}

func getDriveInfo(c web.C, w http.ResponseWriter, rq *http.Request) {

	funcNm := "getDriveInfo"

	ctx := appengine.NewContext(rq)
	u := rq.FormValue("execUser")

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	userEmail := rq.FormValue("fromUser")
	driveId := rq.FormValue("driveId")

	log.Infof(ctx, "fileNm: %s\n", driveId)

	client := getClient(userEmail, rq)

	srv, err := drive.New(client)
	if err != nil {
		log.Infof(ctx, "%s :from drive.New error %v", funcNm, err)
		rj := ResponseJson{Status: "ng", Message: "from drive.New error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	f, err := srv.Files.Get(driveId).Fields("mimeType,createdTime, modifiedByMeTime, permissions").Do()
	if err != nil {
		log.Infof(ctx, "%s :from srv.Files.Get error %v", funcNm, err)
		rj := ResponseJson{Status: "ng", Message: "from srv.Files.Get error"}
		json.NewEncoder(w).Encode(rj)

		return
	}

	log.Infof(ctx, "mimeType: %s\n", f.MimeType)
	log.Infof(ctx, "modifiedByMeTime: %s\n", f.ModifiedByMeTime)
	log.Infof(ctx, "createdTime: %s\n", f.CreatedTime)

	share := ""

	for _, p := range f.Permissions {

		share += p.Type + ":"

		if p.Type == "anyone" {
			share += "anyone(" + p.Role + "):"

			if p.AllowFileDiscovery == true {
				share += "AllowFileDiscovery(true),"
			} else {
				share += "AllowFileDiscovery(false),"
			}

		} else if p.Type == "domain" {
			share += p.Domain + "(" + p.Role + "):"

			if p.AllowFileDiscovery == true {
				share += "AllowFileDiscovery(true),"
			} else {
				share += "AllowFileDiscovery(false),"
			}

		} else {
			share += p.EmailAddress + "(" + p.Role + "),"

		}
	}

	log.Infof(ctx, "share: %s\n", share)

	rj := ResponseJson{Status: "ok", Share: share, ModifiedTime: f.ModifiedByMeTime, CreatedTime: f.CreatedTime, MimeType: f.MimeType}
	json.NewEncoder(w).Encode(rj)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)

}
