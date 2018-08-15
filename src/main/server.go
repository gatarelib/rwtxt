package main

import (
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/schollz/cowyo2/src/db"
	"github.com/schollz/cowyo2/src/utils"
)

var wsupgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func serve() (err error) {
	fs, err := db.New("test.db")
	if err != nil {
		return
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.GET("/", func(cg *gin.Context) {
		cg.HTML(http.StatusOK, "index.html", gin.H{
			"Rendered": utils.RenderMarkdownToHTML(fmt.Sprintf(`

<a href='/%s' class='fr'>New</a>

# cowyo2 

The simplest way to take notes.
			`, strings.ToLower(utils.UUID()))),
		})
	})
	r.GET("/:page", func(cg *gin.Context) {
		page := cg.Param("page")

		if page == "ws" {
			// handle websockets on this page
			c, err := wsupgrader.Upgrade(cg.Writer, cg.Request, nil)
			if err != nil {
				log.Print("upgrade:", err)
				return
			}
			defer c.Close()
			var p Payload
			for {
				err := c.ReadJSON(&p)
				if err != nil {
					log.Println("read:", err)
					break
				}
				log.Printf("recv: %v", p)

				// save it
				if p.ID != "" {
					err = fs.Save(db.File{
						ID:      p.ID,
						Slug:    p.Slug,
						Data:    p.Data,
						Created: time.Now(),
					})
					if err != nil {
						log.Println(err)
					}
				}

				err = c.WriteJSON(Payload{
					Message: "got it",
					Success: true,
				})
				if err != nil {
					log.Println("write:", err)
					break
				}
			}
		} else {
			// handle new page
			log.Printf("loading %s", page)
			havePage, err := fs.Exists(page)
			initialMarkdown := "<a href='#' id='editlink' class='fr'>Edit</a>"
			if err != nil {
				log.Fatal(err)
			}
			var f db.File
			if havePage {
				var files []db.File
				files, err = fs.Get(page)
				if err != nil {
					log.Fatal(err)
				}
				if len(files) > 1 {
					initialMarkdown = fmt.Sprintf("# Found %d '%s'\n\n", len(files), page)
					for _, fi := range files {
						snippet := fi.Data
						if len(snippet) > 50 {
							snippet = snippet[:50]
						}
						reg, _ := regexp.Compile("[^a-z A-Z0-9]+")
						snippet = strings.Replace(snippet, "\n", " ", -1)
						snippet = strings.TrimSpace(reg.ReplaceAllString(snippet, ""))
						initialMarkdown += fmt.Sprintf("\n\n(%s) [%s](/%s) *%s*.", fi.Modified.Format("Mon Jan 2 3:04pm 2006"), fi.ID, fi.ID, snippet)
					}
					cg.HTML(http.StatusOK, "index.html", gin.H{
						"Page":     page,
						"Rendered": utils.RenderMarkdownToHTML(initialMarkdown),
					})
					return
				} else {
					f = files[0]
				}
			} else {
				f = fs.NewFile(page, "# "+page+"\n")
			}
			initialMarkdown += "\n\n" + f.Data

			cg.HTML(http.StatusOK, "index.html", gin.H{
				"Page":     page,
				"Rendered": utils.RenderMarkdownToHTML(initialMarkdown),
				"File":     f,
			})
		}
	})
	log.Printf("running on port 8080")
	r.Run() // listen and serve on 0.0.0.0:8080
	return
}
