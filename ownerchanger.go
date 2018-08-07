package ownerchanger

import (
	"fmt"
	"net/http"

	// Goji
	"github.com/zenazn/goji"
	"github.com/zenazn/goji/web"

	// html
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/user"
	"html/template"
)

func init() {
	http.Handle("/", goji.DefaultMux)

	goji.Get("/", index)

	goji.Post("/userCheck", userCheck)

	goji.Get("/driveListView", index)
	goji.Post("/driveListView", driveListView)

	goji.Post("/driveList", driveList)

	goji.Post("/ownerChange", ownerChange)

	goji.Post("/ownerChangeCron", ownerChangeCron)

	goji.Get("/statusListView", statusListView)
	goji.Post("/statusListView", statusListView)

	goji.Post("/statusList", statusList)

	goji.Post("/statusInfoList", statusInfoList)

	goji.Post("/statusListCursor", statusListCursor)
	goji.Post("/statusInfoListCursor", statusInfoListCursor)

	goji.Post("/taskQueueInfo", taskQueueInfo)

	goji.Post("/taskQueuePurge", taskQueuePurge)

	goji.Get("/settings", settings)

	goji.Get("/uploadView", uploadView)
	goji.Post("/upload", upload)
	goji.Post("/ownerChangeUploadCron", ownerChangeUploadCron)

	goji.Get("/logout", logout)

	goji.Get("/ngListView", ngListView)
	goji.Post("/statusInfoNgListCursor", statusInfoNgListCursor)

	goji.Post("/getNgStatus", getNgStatus)

	goji.Post("/setSuggest", setSuggest)
	goji.Post("/getSuggest", getSuggest)

	goji.Get("/fileDownload", fileDownload)
	goji.Post("/getDriveInfo", getDriveInfo)

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("css"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("js"))))
	http.Handle("/fonts/", http.StripPrefix("/fonts/", http.FileServer(http.Dir("fonts"))))
}

func index(c web.C, w http.ResponseWriter, r *http.Request) {

	funcNm := "index"

	ctx := appengine.NewContext(r)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	data := map[string]interface{}{
		"userEmail": "",
	}

	// page create
	tmpl := template.Must(template.ParseFiles("pages/index.html", "pages/user_select.html"))
	tmpl.Execute(w, data)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)
}

func uploadView(c web.C, w http.ResponseWriter, r *http.Request) {

	funcNm := "upload"

	ctx := appengine.NewContext(r)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	data := map[string]interface{}{
		"userEmail": "",
	}

	tmpl := template.Must(template.ParseFiles("pages/index.html", "pages/upload.html"))
	tmpl.Execute(w, data)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)
}

func logout(c web.C, w http.ResponseWriter, r *http.Request) {

	//funcNm := "logout"

	ctx := appengine.NewContext(r)
	//u := user.Current(ctx)

	logoutUrl, _ := user.LogoutURL(ctx, "/")
	fmt.Fprintf(w, `<a href="%s">Sign out</a>`, logoutUrl)
	return

	/*data := map[string]interface{}{
		"url": logoutUrl,
	}

	tmpl := template.Must(template.ParseFiles("pages/index.html", "pages/logout.html"))
	tmpl.Execute(w, data)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)*/
}

func driveListView(c web.C, w http.ResponseWriter, r *http.Request) {

	funcNm := "driveListView"

	ctx := appengine.NewContext(r)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	data := map[string]interface{}{
		"userEmail": r.FormValue("userEmail"),
	}
	log.Infof(ctx, "start %s ¥n", r.FormValue("userEmail"))

	tmpl := template.Must(template.ParseFiles("pages/index.html", "pages/drive_list.html"))
	tmpl.Execute(w, data)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)
}

func statusListView(c web.C, w http.ResponseWriter, r *http.Request) {

	funcNm := "statusListView"

	ctx := appengine.NewContext(r)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	data := map[string]interface{}{
		"userEmail": "",
	}

	tmpl := template.Must(template.ParseFiles("pages/index.html", "pages/status_list.html"))
	tmpl.Execute(w, data)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)
}

func settings(c web.C, w http.ResponseWriter, r *http.Request) {

	funcNm := "settings"

	ctx := appengine.NewContext(r)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	data := map[string]interface{}{
		"userEmail": "",
	}

	tmpl := template.Must(template.ParseFiles("pages/index.html", "pages/settings.html"))
	tmpl.Execute(w, data)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)
}

func ngListView(c web.C, w http.ResponseWriter, r *http.Request) {

	funcNm := "ngListView"

	ctx := appengine.NewContext(r)
	u := user.Current(ctx)

	// nologin
	if u == nil {
		loginUrl, _ := user.LoginURL(ctx, "/")
		fmt.Fprintf(w, `<a href="%s">Sign in</a>`, loginUrl)
		return
	}

	log.Infof(ctx, "start %s user: %s\n", funcNm, u)

	data := map[string]interface{}{
		"userEmail": "",
	}

	tmpl := template.Must(template.ParseFiles("pages/index.html", "pages/ng_list.html"))
	tmpl.Execute(w, data)

	log.Infof(ctx, "finish %s user: %s\n", funcNm, u)
}
