package main

import (
	"log"
	"fmt"
	"net/http"
	"os"
	"bufio"
	"strings"
	"golang.org/x/net/html"
	//"github.com/russross/blackfriday"
	//"github.com/gin-gonic/gin"
	_ "github.com/heroku/x/hmetrics/onload"
)

var mainPageTmpl = []byte(`
<html>
	<body>
	<b>Welcome to Sparrow's News</b>
	<br><a href="/auth">Sign Up</a> or <a href="/reg">Sign In</a>
	<form action="/" method="post">
		<input type="text" name="search">
		<br><input type="submit" value="Search"></br>
	</form>
	</body>
</html>
`)

func mainPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Write(mainPageTmpl)
		return
	}

	inputQuery := r.FormValue("search")
	w.Write([]byte(search(inputQuery)))
}

var authPageTmpl = []byte(`
<html>
	<body>
	<b>Sign up with your existing account</b>
	<br></br>
	<form action="/auth" method="post">
		Login: <input type="text" name="login">
		<br>Password: <input type="password" name="password"></br>
		<br><input type="submit" value="Login"></br>
	</form>
	</body>
</html>
`)

func authPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Write(authPageTmpl)
		return
	}
	inputLogin := r.FormValue("login")
	inputPassword := r.FormValue("password")
	if !checkPassword(inputLogin, inputPassword) {
		fmt.Fprintln(w, "Uncorrect password or user " + "\"" + inputLogin + "\"" + " not exists")
		return
	}
	fmt.Fprintln(w, "You authorized with account:\nLogin: "+inputLogin+"\nPassword: "+inputPassword)
}

var regPageTmpl = []byte(`
<html>
	<body>
	<b>Register your new account</b>
	<br></br>
	<form action="/reg" method="post">
		Login: <input type="text" name="login">
		Password: <input type="password" name="password">
		<input type="submit" value="Create">
	</form>
	</body>
</html>
`)

func regPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Write(regPageTmpl)
		return
	}
	inputLogin := r.FormValue("login")
	inputPassword := r.FormValue("password")
	if !checkDB(inputLogin, inputPassword) {
		fmt.Fprintln(w, "User: \"" + inputLogin+"\" already exists")
		return
	}
	fmt.Fprintln(w, "You create new account:\nLogin: "+inputLogin+"\nPassword: "+inputPassword)
}


func checkDB(inputLogin string, inputPassword string) bool{
	fileDB, err := os.OpenFile("BD.txt", os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return false
	}
	defer fileDB.Close()
	users := make(map[string]string, 0)
	scanner := bufio.NewScanner(fileDB)
	for scanner.Scan() {
		user := strings.Split(scanner.Text(), " ")
		users[user[0]] = user[1]
	}
	_, mNameExist := users[inputLogin]
	if mNameExist {
		return false
	}
	fileDB.WriteString(inputLogin + " " + inputPassword + "\n")

	return true
}

func checkPassword(inputLogin string, inputPassword string) bool{
	fileDB, err := os.Open("BD.txt")
	if err != nil {
		return false
	}
	defer fileDB.Close()
	users := make(map[string]string, 0)
	scanner := bufio.NewScanner(fileDB)
	for scanner.Scan() {
		user := strings.Split(scanner.Text(), " ")
		users[user[0]] = user[1]
	}
	mPassword, mNameExist := users[inputLogin]
	if mNameExist && mPassword == inputPassword {
		return true
	}

	return false
}

func search(query string) string{
	resp, _ := http.Get("https://habr.com/search/?q=%5B"+query+"%5D")

	result := ""

	z := html.NewTokenizer(resp.Body)

	for {
		tt := z.Next()
		switch {
		case tt == html.ErrorToken:
			return result
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
						result += "<br><a href=" + href + ">" + title + "</a></br>"
						fmt.Printf("<a href=%s>%s</a>\n", href, title)
						break
					}
				}
			}
		}
	}
	return result
}

func main() {

	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	http.HandleFunc("/reg", regPage)

	http.HandleFunc("/auth", authPage)

	http.HandleFunc("/", mainPage)

	//fmt.Println("starting server at :8080")

	http.ListenAndServe(":" + port, nil)
}

/*func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.tmpl.html", nil)
	})

	router.GET("/mark", func(c *gin.Context) {
		c.String(http.StatusOK, string(blackfriday.MarkdownBasic([]byte("**hi!**"))))
	})

	router.Run(":" + port)
}*/
