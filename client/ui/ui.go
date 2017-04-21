// Copyright Banrai LLC. All rights reserved. Use of this source code is
// governed by the license that can be found in the LICENSE file.

// Package ui provides http request handlers for the Pi client WebApp

package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/RogerZhangHS/PiScan/client/database"
	"github.com/mxk/go-sqlite/sqlite3"
	"html/template"
	"io/ioutil"
	"net/http"
	"path"
	"strconv"
	"strings"
)

const (
	// Errors
	BAD_REQUEST = "Sorry, that is an invalid request"
	BAD_POST    = "Sorry, we cannot respond to that request. Please try again."

	// Info messages
	EMAIL_SENT = "The selected items have been sent to your email address"

	// urls
	HOME_URL    = "/scanned/"
	ACCOUNT_URL = "/account/"
)

var (
	TEMPLATE_LIST = func(templatesFolder string, templateFiles []string) []string {
		t := make([]string, 0)
		for _, f := range templateFiles {
			t = append(t, path.Join(templatesFolder, f))
		}
		return t
	}

	UNSUPPORTED_TEMPLATE_FILE = "browser_not_supported.html"

	ITEM_LIST_TEMPLATE_FILES = []string{"items.html", "head.html", "navigation_tabs.html", "actions.html", "modal.html", "scripts.html"}
	ITEM_EDIT_TEMPLATE_FILES = []string{"define_item.html", "head.html", "scripts.html"}

	ITEM_LIST_TEMPLATES *template.Template
	ITEM_EDIT_TEMPLATES *template.Template

	TEMPLATES_INITIALIZED = false
)

// 用来进行重定向的函数 (string)
func Redirect(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}
}

// Respond to requests using HTML templates and the standard Content-Type (i.e., "text/html")
func MakeHTMLHandler(fn func(http.ResponseWriter, *http.Request, database.ConnCoordinates, ...interface{}), db database.ConnCoordinates, opts ...interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r, db, opts...)
	}
}

// Show the static template for unsupported browsers
func UnsupportedBrowserHandler(templatesFolder string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadFile(path.Join(templatesFolder, UNSUPPORTED_TEMPLATE_FILE))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, string(body))
	}
}

// Respond to requests that are not "text/html" Content-Types (e.g., for ajax calls)
func MakeHandler(fn func(*http.Request, database.ConnCoordinates, ...interface{}) string, db database.ConnCoordinates, mediaType string, opts ...interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", fmt.Sprintf("%s; charset=utf-8", mediaType))
		data := fn(r, db, opts...)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		fmt.Fprintf(w, data)
	}
}

/* JSON response struct */
type AjaxAck struct {
	Message string `json:"msg"`
	Error   string `json:"err,omitempty"`
}

/* HTML template structs */
type ActiveTab struct {
	Scanned    bool
	Submission bool
	ShowTabs   bool
}

type Action struct {
	Icon   string
	Link   string
	Action string
}

type StudentPage struct {
	Title       string
	ActiveTab   *ActiveTab
	Actions     []*Action
	Students       []*database.Student
	Scanned     bool
	PageMessage string
}

type StudentForm struct {
	StuName      string
	Item         *database.Student
	CancelUrl    string
	FormError    string
	FormMessage  string
	Unregistered bool
}

/* General db access functions */

// getStudents returns a list of all students and submitted students, and the correct
// corresponding options for the HTML page template
func getStudents(w http.ResponseWriter, r *http.Request, dbCoords database.ConnCoordinates, submitted bool) {
	// 尝试连接至本地数据库
	db, err := database.InitializeDB(dbCoords)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// 根据具体情况确定获取数据库内条目的函数
	fetch := func(db *sqlite3.Conn) ([]*database.Student, error) {
		if submitted {
			return database.GetSignedStudents(db)
		} else {
			return database.GetAllStudents(db)
		}
	}

	// get all the desired students for this Account
	students := make([]*database.Student, 0)
	studentsList, studentsErr := fetch(db)
	if studentsErr != nil {
		http.Error(w, studentsErr.Error(), http.StatusInternalServerError)
		return
	}
	for _, student := range studentsList {
		students = append(students, student)
	}

	// actions
	actions := make([]*Action, 0)
	if submitted {
		actions = append(actions, &Action{Link: "/unsubmit/", Icon: "fa fa-star-o", Action: "将学生从提交名单中移除"})
	} else {
		actions = append(actions, &Action{Link: "/submit/", Icon: "fa fa-star", Action: "将学生加入提交名单中"})
	}
	actions = append(actions, &Action{Link: "/delete/", Icon: "fa fa-trash", Action: "删除该学生"})

	// define the page title
	var titleBuffer bytes.Buffer
	if submitted {
		titleBuffer.WriteString("已提交 | ")
	} else {
		titleBuffer.WriteString("全部 | ")
	}
	titleBuffer.WriteString(" 学生")

	p := &StudentPage{Title: titleBuffer.String(),
		Scanned:         !submitted,
		ActiveTab:       &ActiveTab{Scanned: !submitted, Submission: submitted, ShowTabs: true},
		Actions:         actions,
		Students:        students}

	// check for any message to display on page load
	r.ParseForm()
	if msg, exists := r.Form["ack"]; exists {
		ackType := strings.Join(msg, "")
		if ackType == "email" {
			p.PageMessage = EMAIL_SENT
		}
	}

	renderItemListTemplate(w, p)
}

// deleteItem attempts to lookup and remove the Item for the Account and
// Item.Id combination, returning a bool on success/fail, and the db lookup
// error (if any)
func deleteItem(db *sqlite3.Conn, stuid string) (bool, error) {
	result := false

	student, studentErr := database.GetSingleItem(db, stuid)
	if studentErr == nil {
		if student.stuid == stuid {
			database.Delete(db,stuid)
			result = true
		}
	}

	return result, studentErr
}

// processItems fetches all the Items for the given Account, and the compares
// them to the id list posted from the form. All the matches get applied
// the given function: delete, favorite, unfavorite, etc.
func processItems(w http.ResponseWriter, r *http.Request, dbCoords database.ConnCoordinates, fn func(*database.Item, *sqlite3.Conn), successTarget string) {
	// attempt to connect to the db
	db, err := database.InitializeDB(dbCoords)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// get the Account for this request
	acc, accErr := database.GetDesignatedAccount(db)
	if accErr != nil {
		http.Error(w, accErr.Error(), http.StatusInternalServerError)
		return
	}

	// get all the Items for this Account
	// and store them in a map by their Idscanned/
	items, itemsErr := database.GetItems(db, acc)
	if itemsErr != nil {
		http.Error(w, itemsErr.Error(), http.StatusInternalServerError)
		return
	}
	accountItems := make(map[int64]*database.Item)
	for _, item := range items {
		accountItems[item.Id] = item
	}

	// get the list of item ids from the POST values
	// and apply the processing function
	if "POST" == r.Method {
		r.ParseForm()
		if idVals, exists := r.PostForm["item"]; exists {
			for _, idString := range idVals {
				id, idErr := strconv.ParseInt(idString, 10, 64)
				if idErr == nil {
					if accountItem, ok := accountItems[id]; ok {
						fn(accountItem, db)
					}
				}
			}
		}
	}

	// finally, return home, to the scanned items list
	http.Redirect(w, r, successTarget, http.StatusFound)
}

/* HTML Response Functions (via templates) */

func renderItemListTemplate(w http.ResponseWriter, p *StudentPage) {
	if TEMPLATES_INITIALIZED {
		ITEM_LIST_TEMPLATES.Execute(w, p)
	}
}

func renderItemEditTemplate(w http.ResponseWriter, f *StudentForm) {
	if TEMPLATES_INITIALIZED {
		ITEM_EDIT_TEMPLATES.Execute(w, f)
	}
}

// InitializeTemplates confirms the given folder string leads to the html
// template files, otherwise templates.Must() will complain
func InitializeTemplates(folder string) {
	ITEM_LIST_TEMPLATES = template.Must(template.ParseFiles(TEMPLATE_LIST(folder, ITEM_LIST_TEMPLATE_FILES)...))
	ITEM_EDIT_TEMPLATES = template.Must(template.ParseFiles(TEMPLATE_LIST(folder, ITEM_EDIT_TEMPLATE_FILES)...))
	ACCOUNT_EDIT_TEMPLATES = template.Must(template.ParseFiles(TEMPLATE_LIST(folder, ACCOUNT_EDIT_TEMPLATE_FILES)...))
	TEMPLATES_INITIALIZED = true
}

// ScannedItems returns all the products scanned, favorited or not, barcode
// lookup successful or not
func ScannedItems(w http.ResponseWriter, r *http.Request, db database.ConnCoordinates, opts ...interface{}) {
	getStudents(w, r, db, false)
}

// FavoritedItems returns all the products scanned and favorited by this
// Account
func FavoritedItems(w http.ResponseWriter, r *http.Request, db database.ConnCoordinates, opts ...interface{}) {
	getStudents(w, r, db, true)
}

// DeleteItems accepts a form post of one or more Item.Id values, and
// attempts to remove them from the client db. Unless it hits a critical
// error, it returns home, to the list of scanned items
func DeleteItems(w http.ResponseWriter, r *http.Request, dbCoords database.ConnCoordinates, opts ...interface{}) {
	del := func(i *database.Item, db *sqlite3.Conn) {
		i.Delete(db)
	}
	processItems(w, r, dbCoords, del, "/")
}

// FavoriteItems accepts a form post of one or more Item.Id values, and
// attempts to change their status in the client db to 'favorite'
func FavoriteItems(w http.ResponseWriter, r *http.Request, dbCoords database.ConnCoordinates, opts ...interface{}) {
	fav := func(i *database.Item, db *sqlite3.Conn) {
		i.Favorite(db)
	}
	processItems(w, r, dbCoords, fav, "/favorites/")
}

// UnfavoriteItems accepts a form post of one or more Item.Id values,
// and attempts to change their status in the client db to not 'favorite'
func UnfavoriteItems(w http.ResponseWriter, r *http.Request, dbCoords database.ConnCoordinates, opts ...interface{}) {
	fav := func(i *database.Item, db *sqlite3.Conn) {
		i.Unfavorite(db)
	}
	processItems(w, r, dbCoords, fav, "/favorites/")
}

/* Ajax Response Functions (as strings via MakeHandler) */

// RemoveSingleItem looks up the single item represented by the itemId form
// post variable, and attempts to delete it, if it exists. The reply is a
// jsonified string, passed back to MakeHandler() to be coupled with the
// right mime type
func RemoveSingleItem(r *http.Request, dbCoords database.ConnCoordinates, opts ...interface{}) string {
	// prepare the ajax reply object
	ack := AjaxAck{Message: "", Error: ""}

	// attempt to connect to the db
	db, err := database.InitializeDB(dbCoords)
	if err != nil {
		ack.Error = err.Error()
	}
	defer db.Close()

	if err == nil {
		// get the Account for this request
		acc, accErr := database.GetDesignatedAccount(db)
		if accErr != nil {
			ack.Error = accErr.Error()
		}

		// find the specific Item to remove
		// get the item id from the POST values
		if "POST" == r.Method {
			r.ParseForm()
			if idVal, exists := r.PostForm["itemId"]; exists {
				if len(idVal) > 0 {
					id, idErr := strconv.ParseInt(idVal[0], 10, 64)
					if idErr != nil {
						ack.Error = idErr.Error()
					} else {
						deleteSuccess, deleteErr := deleteItem(db, acc, id)
						if deleteSuccess {
							ack.Message = "Ok"
						} else {
							if deleteErr != nil {
								ack.Error = deleteErr.Error()
							} else {
								ack.Error = "No such item"
							}
						}
					}
				} else {
					ack.Error = "Missing item id"
				}
			} else {
				ack.Error = BAD_POST
			}
		} else {
			ack.Error = BAD_REQUEST
		}
	}

	// convert the ajax reply object to json
	ackObj, ackObjErr := json.Marshal(ack)
	if ackObjErr != nil {
		return ackObjErr.Error()
	}
	return string(ackObj)
}
