package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"path/filepath"

	"github.com/dotzero/pad/service"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
)

// App is a Pad app
type App struct {
	Config *Configuration
	Router *chi.Mux
	Redis  *service.Redis
	HashID *service.HashID
}

// Pad is a Pad data
type Pad struct {
	Name    string
	Content string
}

func (a *App) Initialize(cfg *Configuration, pwd string) {
	a.Config = cfg
	a.Router = chi.NewRouter()
	a.Redis = service.NewRedisClient(cfg.RedisURI, cfg.RedisPrefix)
	a.HashID = service.NewHashID(cfg.Salt)
	a.initializeMiddlewares()
	a.initializeRoutes()
	a.initializeStatic(pwd)
}

func (a *App) Run() {
	log.Printf("Listen at: 0.0.0.0:%s\n", a.Config.Port)
	log.Fatal(http.ListenAndServe(":"+a.Config.Port, a.Router))
}

func (a *App) initializeMiddlewares() {
	a.Router.Use(middleware.Logger)
	a.Router.Use(middleware.NoCache)
	a.Router.Use(middleware.RealIP)
	a.Router.Use(middleware.Recoverer)
	a.Router.Use(middleware.RedirectSlashes)
}

func (a *App) initializeRoutes() {
	a.Router.Get("/", a.createPad)
	a.Router.Route("/{padname}", func(r chi.Router) {
		r.Get("/", a.getPad)
		r.Post("/", a.updatePad)
	})
}

func (a *App) initializeStatic(pwd string) {
	static := filepath.Join(pwd, "static")

	// a.Router.Get("/favicon.ico", a.createPad)

	func faviconHandler(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "relative/path/to/favicon.ico")
	}

	handleStatics(a.Router, "/static", http.Dir(static))
}

func (a *App) createPad(w http.ResponseWriter, r *http.Request) {
	cnt, err := a.Redis.GetNextCounter()
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	hash := a.HashID.Encode(cnt)
	http.Redirect(w, r, "/"+hash, 301)
}

func (a *App) getPad(w http.ResponseWriter, r *http.Request) {
	padname := chi.URLParam(r, "padname")
	content := a.Redis.GetPad(padname)

	tpl := template.New("main")
	tpl, _ = tpl.ParseFiles("templates/main.html")
	tpl.Execute(w, Pad{
		Name:    padname,
		Content: content,
	})
}

func (a *App) updatePad(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	padname := chi.URLParam(r, "padname")
	content := r.Form.Get("t")

	a.Redis.SetPad(padname, content)

	respondWithJSON(w, 200, map[string]string{
		"message": "ok",
		"padname": padname,
	})
}

// func faviconHandler(w http.ResponseWriter, r *http.Request) {
// 	http.ServeFile(w, r, "relative/path/to/favicon.ico")
// }

func staticHandler(r chi.Router, path string, root http.FileSystem) {
	fs := http.StripPrefix(path, http.FileServer(root))

	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", 301).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.ServeHTTP(w, r)
	}))
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}
