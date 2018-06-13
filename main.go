package main

import (
"database/sql"
"fmt"
"html/template"
"net/http"
"golang.org/x/net/html"
"github.com/gorilla/mux"

_ "github.com/go-sql-driver/mysql"
	"os"
	"log"
	"time"
	"strings"
	"crypto/sha256"
)

type User struct {
	Id          int
	Login       string
	Password	string
	Tags     	sql.NullString
}

type Article struct {
	Title string
	Href string
	Description string
	Date string
}

type Request struct {
	Id          int
	Login       sql.NullString
	Tag     	string
}

type Session struct {
	Authorized bool
	User string
	Tags []string
	Articles []Article
}

type Handler struct {
	DB   *sql.DB
	Tmpl *template.Template
}

func (h *Handler) Users(w http.ResponseWriter, r *http.Request) {

	users := []*User{}

	rows, err := h.DB.Query("SELECT id, login, password, tags FROM users")
	__err_panic(err)
	for rows.Next() {
		post := &User{}
		err = rows.Scan(&post.Id, &post.Login, &post.Password, &post.Tags)
		__err_panic(err)
		users = append(users, post)
	}
	// надо закрывать соединение, иначе будет течь
	rows.Close()

	err = h.Tmpl.ExecuteTemplate(w, "users.html", struct {
		Users []*User
	}{
		Users: users,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) AddUser(w http.ResponseWriter, r *http.Request) {
	// в целям упрощения примера пропущена валидация
	result, err := h.DB.Exec(
		"INSERT INTO users (`login`, `password`, `tags`) VALUES (?, ?, ?)",
		r.FormValue("login"),
		hash(r.FormValue("password"), r.FormValue("login")),
		r.FormValue("tags"),
	)
	__err_panic(err)

	affected, err := result.RowsAffected()
	__err_panic(err)
	lastID, err := result.LastInsertId()
	__err_panic(err)

	fmt.Println("Insert - RowsAffected", affected, "LastInsertId: ", lastID)

	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) AddUserForm(w http.ResponseWriter, r *http.Request) {
	err := h.Tmpl.ExecuteTemplate(w, "reg.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func search(query string) []Article{
	resp, _ := http.Get("https://habr.com/search/?target_type=posts&q=%5B"+query+"%5D&order_by=date")

	articles := []Article{}
	z := html.NewTokenizer(resp.Body)
	for {
		tt := z.Next()
		switch {
		case tt == html.ErrorToken:
			return articles
		case tt == html.StartTagToken:
			t := z.Token()
			if t.Data == "a" {
				href := ""
				title := ""
				for _, a := range t.Attr {
					if a.Key == "href" {
						href = a.Val
					}
					if a.Key == "class" && a.Val == "post__title_link" {
						tt = z.Next()
						t := z.Token()
						title = t.Data
						articles = append(articles, Article{Title: title, Href: href})
						break
					}
				}
			}
		}
	}
	fmt.Printf("%#v \n", articles)
	return articles
}

func (h *Handler) MainPost(w http.ResponseWriter, r *http.Request) {
	session := Session{
		Authorized: false,
	}
	inputQuery := r.FormValue("search")
	login, err := r.Cookie("login")
	loggedIn := (err != http.ErrNoCookie)

	if loggedIn {
		tags, err := r.Cookie("tags")
		session.User = login.Value
		session.Tags = parseTags(tags.Value)
		session.Authorized = true
		settedCookies := (err != http.ErrNoCookie)
		if !settedCookies {
			session.Tags = nil
		}
		_, err = h.DB.Exec(
			"INSERT INTO requests (`login`, `tag`) VALUES (?, ?)",
			login.Value,
			inputQuery,
		)
	} else {
		_, err = h.DB.Exec(
			"INSERT INTO requests (`tag`) VALUES (?)",
			inputQuery,
		)
	}

	__err_panic(err)

	session.Articles = search(inputQuery)

	err = h.Tmpl.ExecuteTemplate(w, "main.html", session)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func parseTags(tags string) []string {
	result := strings.Split(strings.Trim(tags, `"`), ",")
	for idx, tag := range result {
		result[idx] = strings.Trim(tag, " ")
	}
	return result
}

func (h *Handler) Main(w http.ResponseWriter, r *http.Request) {

	session := Session{
		Authorized: false,
	}

	login, err := r.Cookie("login")
	loggedIn := (err != http.ErrNoCookie)
	if loggedIn {
		tags, err := r.Cookie("tags")
		session.User = login.Value
		session.Tags = parseTags(tags.Value)
		session.Authorized = true
		settedCookies := (err != http.ErrNoCookie)
		if !settedCookies {
			session.Tags = nil
		}

	}

	for _, tag := range session.Tags {
		searchResult := search(tag)
		session.Articles = append(session.Articles, searchResult...)
	}

	err = h.Tmpl.ExecuteTemplate(w, "main.html", session)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) AuthForm(w http.ResponseWriter, r *http.Request) {
	type Flag struct {
		Unfound bool
	}
	flag := Flag{Unfound: false}
	err := h.Tmpl.ExecuteTemplate(w, "auth.html", flag)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Auth(w http.ResponseWriter, r *http.Request) {

	inputLogin := r.FormValue("login")
	inputPassword := hash(r.FormValue("password"), r.FormValue("login"))

	query := fmt.Sprintf("SELECT login, tags FROM users WHERE login = '%s' and password = '%s'", inputLogin, inputPassword)
	rows, err := h.DB.Query(query)
	__err_panic(err)
	if rows.Next() {
		user := &User{}
		err = rows.Scan(&user.Login, &user.Tags)
		rows.Close()
		expiration := time.Now().Add(10 * time.Hour)
		cookie := http.Cookie{
			Name:    "login",
			Value:   user.Login,
			Expires: expiration,
		}

		http.SetCookie(w, &cookie)
		cookie = http.Cookie{
			Name:    "tags",
			Value:   user.Tags.String,
			Expires: expiration,
		}
		http.SetCookie(w, &cookie)
		http.Redirect(w, r, "/", http.StatusFound)
		fmt.Println("Correct password!")

	} else {
		rows.Close()
		type Flag struct {
			Unfound bool
		}
		flag := Flag{Unfound: true}
		err := h.Tmpl.ExecuteTemplate(w, "auth.html", flag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Println("Incorrect password!!!")
	}
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	login, _ := r.Cookie("login")
	tags, _ := r.Cookie("tags")

	login.Expires = time.Now().AddDate(0, 0, -1)
	tags.Expires = time.Now().AddDate(0, 0, -1)

	http.SetCookie(w, login)
	http.SetCookie(w, tags)
	http.Redirect(w, r, "/", http.StatusFound)
}


func (h *Handler) EditUser(w http.ResponseWriter, r *http.Request) {
	login, _ := r.Cookie("login")
	rows, err := h.DB.Query("SELECT id, login, tags FROM users WHERE login = ?", login.Value)
	__err_panic(err)
	if rows.Next() {
		user := &User{}
		err = rows.Scan(&user.Id, &user.Login, &user.Tags)
		err = h.Tmpl.ExecuteTemplate(w, "edituser.html", user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	return
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {


	// в целям упрощения примера пропущена валидация
	result, err := h.DB.Exec(
		"UPDATE users SET"+
			"`login` = ?"+
			",`password` = ?"+
			",`tags` = ?"+
			"WHERE id = ?",
		r.FormValue("login"),
		hash(r.FormValue("password"), r.FormValue("login")),
		r.FormValue("tags"),
		r.FormValue("id"),
	)

	__err_panic(err)

	affected, err := result.RowsAffected()
	__err_panic(err)

	fmt.Println("Update - RowsAffected", affected)

	expiration := time.Now().Add(10 * time.Hour)
	cookie := http.Cookie{
		Name:    "login",
		Value:   r.FormValue("login"),
		Expires: expiration,
	}

	http.SetCookie(w, &cookie)
	cookie = http.Cookie{
		Name:    "tags",
		Value:   r.FormValue("tags"),
		Expires: expiration,
	}
	http.SetCookie(w, &cookie)

	http.Redirect(w, r, "/", http.StatusFound)
}

func hash(str string, salt string) string {
	res := fmt.Sprintf("%x", sha256.Sum256([]byte(str)))
	res = fmt.Sprintf("%x%s", sha256.Sum256([]byte(res)), salt)
	return res
}

func (h *Handler) Requests(w http.ResponseWriter, r *http.Request) {

	requests := []*Request{}

	rows, err := h.DB.Query("SELECT id, login, tag FROM requests")
	__err_panic(err)
	for rows.Next() {
		post := &Request{}
		err = rows.Scan(&post.Id, &post.Login, &post.Tag)
		__err_panic(err)
		requests = append(requests, post)
	}
	// надо закрывать соединение, иначе будет течь
	rows.Close()

	err = h.Tmpl.ExecuteTemplate(w, "requests.html", struct {
		Requests []*Request
	}{
		Requests: requests,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}


func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	// основные настройки к базе
	dsn := "pb9zojk8hpvfjwes:u4pv3tjbu4ztvzf3@tcp(nuskkyrsgmn5rw8c.cbetxkdyhwsb.us-east-1.rds.amazonaws.com:3306)/jn1wuqllqn8o60ya?"
	// указываем кодировку
	dsn += "&charset=utf8"
	// отказываемся от prapared statements
	// параметры подставляются сразу
	dsn += "&interpolateParams=true"

	db, err := sql.Open("mysql", dsn)

	db.SetMaxOpenConns(10)

	err = db.Ping() // вот тут будет первое подключение к базе
	if err != nil {
		panic(err)
	}

	handlers := &Handler{
		DB:   db,
		Tmpl: template.Must(template.ParseGlob("crud_templates/*")),
	}

	// в целям упрощения примера пропущена авторизация и csrf
	r := mux.NewRouter()
	r.HandleFunc("/", handlers.Main).Methods("GET")
	r.HandleFunc("/", handlers.MainPost).Methods("POST")
	r.HandleFunc("/users", handlers.Users).Methods("GET")
	r.HandleFunc("/reg", handlers.AddUserForm).Methods("GET")
	r.HandleFunc("/reg", handlers.AddUser).Methods("POST")
	r.HandleFunc("/auth", handlers.AuthForm).Methods("GET")
	r.HandleFunc("/auth", handlers.Auth).Methods("POST")
	r.HandleFunc("/logout", handlers.Logout).Methods("GET")
	r.HandleFunc("/edituser", handlers.EditUser).Methods("GET")
	r.HandleFunc("/edituser", handlers.UpdateUser).Methods("POST")
	r.HandleFunc("/requests", handlers.Requests).Methods("GET")
	fmt.Println("starting server at :" + port)
	http.ListenAndServe(":" + port, r)
}

// не используйте такой код в прошакшене
// ошибка должна всегда явно обрабатываться
func __err_panic(err error) {
	if err != nil {
		panic(err)
	}
}

