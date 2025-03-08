package main 
import (
  "net/http"
  "net/url"
  "strconv"
  "crypto/rand"
  "encoding/base64"
  "crypto/sha256"
  "html/template"
  "fmt"
  "database/sql"
  "strings"
  _ "github.com/mattn/go-sqlite3"
)

type Message struct {
  Text string 
  AuthorUsername string  
}

type Chat struct {
  ChatName string
  ChatID string 
  Messages []Message
}

var db *sql.DB 

func AuthenticateUser(r *http.Request) (ID , username string ){
  
  sessionID , err := r.Cookie("sessionID")
  if err != nil {

    if err == http.ErrNoCookie {
      fmt.Fprint(w , "you are a guest")
    } else {
      fmt.Fprint(w , err)
    }
    return "" , "" , err 
  }
  
  var username string
  var userID string 

  row := db.QueryRow("SELECT username AND ID FROM users WHERE ID = ( SELECT userID FROM sessions WHERE sessionID = ? ) ;" , hashString(sessionID.Value))

  err = row.Scan(&username)

  if err != nil {
    if err == sql.ErrNoRows {
      fmt.Fprint(w , "this account does not exist ! <a href='/login' > login ? </a> ")
    } else {
      fmt.Fprint(w , err)
    }
    return "" , "" , err 
  }

  return username , userID , nil 
}



func GenerateRadnomString() (string) {
  randomBytes := make([]byte , 9 )
  _ , err := rand.Read(randomBytes)
  
  if err != nil {
    fmt.Println(err)
    return ""
  }

  return base64.RawURLEncoding.EncodeToString(randomBytes)[:12]

}

func hashString(str string ) (string) {
  h := sha256.New() 
  h.Write([]byte(str))

  return fmt.Sprintf("%x" , h.Sum(nil))
}

func chatRoute(w http.ResponseWriter , r *http.Request ) {

  inputtedId := strings.TrimPrefix(r.URL.Path , "/chat/")

  row := db.QueryRow(" SELECT ID FROM Chats WHERE ID = ? ;" , inputtedId)

  err := row.Scan(&inputtedId)

  if err != nil {
    if err == sql.ErrNoRows {
      fmt.Fprint(w , "this chat does not exist.")
      return 
    } else {
      fmt.Fprint(w , err)
      return 
    }
  }

  var Messages []Message
  var rows *sql.Rows 

  rows , err = db.Query("SELECT Text FROM Messages WHERE ChatID = ? ;" , inputtedId)

  if err != nil {
    fmt.Fprint(w , err)
    return 
  }

  for rows.Next() {
    authorUsername := "Default user"
    var Text string 
    
    err = rows.Scan(&Text)

    if err != nil {
      fmt.Fprint(w , err)
      return 
    }

    currMessage := Message{
      Text : Text , 
      AuthorUsername : authorUsername , 
    }

    Messages = append(Messages , currMessage)

  }

  htmlData := struct{
    Messages []Message
  }{
    Messages : Messages , 
  }

  var tmpl *template.Template


  tmpl , err = template.ParseFiles("templates/chats.html" , "templates/base.html")

  if err != nil {
    fmt.Fprint(w , err)
    return 
  }
  
  err = tmpl.Execute(w , htmlData) 

  if err != nil {
    fmt.Fprint(w , err)
    return 
  } 

} 

func loginRoute(w http.ResponseWriter , r *http.Request) {
  if r.Method == "GET" {
    tmp , err := template.ParseFiles("templates/login.html" , "templates/base.html")

    if err != nil {
      fmt.Fprint(w , err)
      return 
    }

    err = tmp.Execute(w , nil )
    return 
  }
  var userID int 

  r.ParseForm() 

  
  username := r.FormValue("username")
  password := hashString(r.FormValue("password")) // when done coding this route , please make sure to use hashString on the password

  if username == "" || password == "" {
    fmt.Fprint(w , "you cannot leave either of the fields empty ")
    return 
  }

  row := db.QueryRow(" SELECT ID FROM Users WHERE username = ? AND password = ? ; " , username , password)

  err := row.Scan(&userID)

  if err != nil {
    
    if err == sql.ErrNoRows {
      fmt.Fprint(w , "this user does not exist !")
      return 
    }

    fmt.Fprint(w , err)
    return 
  }

  sessionID := GenerateRadnomString()

  fmt.Println(sessionID)

  _ , err = db.Exec("INSERT INTO sessions (sessionID , userID ) VALUES (? , ? ) ; " , hashString(sessionID) , userID )

  if err != nil {
    fmt.Fprint(w , err) 
    return 
  }
  
  userCookie := http.Cookie{
    Name : "sessionID" , 
    Value : sessionID , 
    HttpOnly : true , 
    Secure : true ,  
  }

  http.SetCookie(w , &userCookie )

  http.Redirect(w , r , "/" , http.StatusFound )

}

func signup(w http.ResponseWriter , r *http.Request ){
  if r.Method == "GET" {
    
    tmp , err := template.ParseFiles("templates/signup.html" , "templates/base.html")
    
    if err != nil {
      fmt.Fprint(w , err)
      return 
    }

    err = tmp.Execute(w , nil )

    if err != nil {
      fmt.Fprint(w , err)
      return 
    }
    

    return 
  }
  
  r.ParseForm() 

  username := r.FormValue("username")
  password := r.FormValue("password")

  if username == "" || password == "" {
    fmt.Fprint(w , "you cannot leave username or password as blank !")
    return 
  }

  var exists string 

  row := db.QueryRow(" SELECT exists( SELECT ID FROM Users WHERE username = ? AND password = ? ) ;  " , username , hashString(password))

  err := row.Scan(&exists )

  if err != nil {
    fmt.Fprint(w , err)
    return 
  }
  
  if exists == "1" {
    fmt.Fprint(w , "user already exists , <a href='/login' > login ? </a> ")
    return 
  }

  _ , err = db.Exec(" INSERT INTO Users ( username , password ) VALUES (? , ? ) ; " , username , hashString(password) )

  if err != nil {
    fmt.Fprint(w , err)
    return 
  }

  fmt.Fprint(w , "user added successfully ")
  
}



func sendMessageRoute(w http.ResponseWriter , r *http.Request) {

  if r.Method == "GET" {
    fmt.Fprint( w , "you can not access this route")
    return 
  }

  r.ParseForm() 
  messageText := r.FormValue("messageText")
  RefererURL , err := url.Parse(r.Referer())

  if err != nil {
    fmt.Fprint(w , "invalid request ! ")
    return 
  }

  if messageText == "" {
    fmt.Fprint(w , "text field should not be empty")
    return  
  }

  if RefererURL.Path == "" {
    fmt.Fprint(w , "invalid , a message without a chatID ")
    return 
  }
  

  _ , err = db.Exec(
    "INSERT INTO Messages (Text , ChatID ) VALUES (? , ? ) ; ",
  messageText , strings.TrimPrefix(RefererURL.Path , 
  "/chat/"))
  
  if err != nil {
    fmt.Fprint(w , err )
    return 
  }

  http.Redirect(w , r , RefererURL.Path , http.StatusFound )

}



func homeRoute(w http.ResponseWriter , r *http.Request ) {

  
  var Chats []Chat
  var rows *sql.Rows

  rows , err = db.Query("SELECT ChatName , ID FROM Chats  ; ")
  
  if err != nil {
    fmt.Println(err)
    return 
  }

  for rows.Next() {
    var chatName string  
    var ID int

    err := rows.Scan(&chatName , &ID )

    if err != nil {
      fmt.Println( err)
      return 
    }
    
    currChat := Chat{
      ChatName : chatName , 
      ChatID : strconv.Itoa(ID) , 
    }

    Chats = append(Chats , currChat)



  }

  htmlData := struct {
    Chats []Chat
    Username string 
  }{  Chats , username , }
    
  var tmpl *template.Template

  tmpl , err = template.ParseFiles("templates/mainRoute.html" , "templates/base.html")
  if err != nil {
    fmt.Fprint(w , err)
    return 
  }

  err = tmpl.Execute(w , htmlData) 
  
  if err != nil {
    fmt.Fprint(w , err)
    return 
  }

}

func main() {
  var err error
  db , err = sql.Open("sqlite3" , "main.db")
  
  if err != nil {
    fmt.Println(err)
    panic(err)
  }

  defer db.Close() 

   http.HandleFunc("/chat/" , chatRoute)
   http.HandleFunc("/login" , loginRoute)
   http.HandleFunc("/signup" , signup)
   http.HandleFunc("/" , homeRoute) 
   http.HandleFunc("/sendMessage" , sendMessageRoute)


   http.ListenAndServe(":8080" , nil )
}
