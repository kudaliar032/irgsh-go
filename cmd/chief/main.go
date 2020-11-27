package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	machinery "github.com/RichardKnop/machinery/v1"
	machineryConfig "github.com/RichardKnop/machinery/v1/config"
	"github.com/urfave/cli"

	"github.com/blankon/irgsh-go/internal/config"

	artifactEndpoint "github.com/blankon/irgsh-go/internal/artifact/endpoint"
	artifactRepo "github.com/blankon/irgsh-go/internal/artifact/repo"
	artifactService "github.com/blankon/irgsh-go/internal/artifact/service"
)

var (
	app     *cli.App
	server  *machinery.Server
	version string

	irgshConfig config.IrgshConfig

	artifactHTTPEndpoint *artifactEndpoint.ArtifactHTTPEndpoint
)

type Submission struct {
	TaskUUID       string    `json:"taskUUID"`
	Timestamp      time.Time `json:"timestamp"`
	PackageName    string    `json:"packageName"`
	PackageURL     string    `json:"packageUrl"`
	SourceURL      string    `json:"sourceUrl"`
	Maintainer     string    `json:"maintainer"`
	Component      string    `json:"component"`
	IsExperimental bool      `json:"isExperimental"`
	Tarball        string    `json:"tarball"`
}

type ArtifactsPayloadResponse struct {
	Data []string `json:"data"`
}

type SubmitPayloadResponse struct {
	PipelineId string   `json:"pipelineId"`
	Jobs       []string `json:"jobs,omitempty"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	var err error
	irgshConfig, err = config.LoadConfig()
	if err != nil {
		log.Fatalln(err)
	}

	// Prepare workdir
	err = os.MkdirAll(irgshConfig.Chief.Workdir, 0755)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(irgshConfig.Chief.Workdir)

	artifactHTTPEndpoint = artifactEndpoint.NewArtifactHTTPEndpoint(
		artifactService.NewArtifactService(
			artifactRepo.NewFileRepo(irgshConfig.Chief.Workdir)))

	app = cli.NewApp()
	app.Name = "irgsh-go"
	app.Usage = "irgsh-go distributed packager"
	app.Author = "BlankOn Developer"
	app.Email = "blankon-dev@googlegroups.com"
	app.Version = version

	app.Action = func(c *cli.Context) error {
		server, err = machinery.NewServer(
			&machineryConfig.Config{
				Broker:        irgshConfig.Redis,
				ResultBackend: irgshConfig.Redis,
				DefaultQueue:  "irgsh",
			},
		)
		if err != nil {
			fmt.Println("Could not create server : " + err.Error())
		}

		serve()

		return nil
	}
	app.Run(os.Args)

}

func serve() {
	http.HandleFunc("/", indexHandler)

	// APIs
	http.HandleFunc("/api/v1/artifacts", artifactHTTPEndpoint.GetArtifactListHandler)
	http.HandleFunc("/api/v1/submit", PackageSubmitHandler)
	http.HandleFunc("/api/v1/status", BuildStatusHandler)
	http.HandleFunc("/api/v1/artifact-upload", artifactUploadHandler())
	http.HandleFunc("/api/v1/log-upload", logUploadHandler())
	http.HandleFunc("/api/v1/build-iso", BuildISOHandler)
	http.HandleFunc("/api/v1/version", VersionHandler)

	// Pages
	http.HandleFunc("/maintainers", MaintainersHandler)
	// Static file routes
	artifactFs := http.FileServer(http.Dir(irgshConfig.Chief.Workdir + "/artifacts"))
	http.Handle("/artifacts/", http.StripPrefix("/artifacts/", artifactFs))
	logFs := http.FileServer(http.Dir(irgshConfig.Chief.Workdir + "/logs"))
	http.Handle("/logs/", http.StripPrefix("/logs/", logFs))

	port := os.Getenv("PORT")
	if len(port) < 1 {
		port = "8080"
	}
	log.Println("irgsh-go chief now live on port " + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	resp := "<div style=\"font-family:monospace !important\">"
	resp += "&nbsp;_&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;"
	resp += "&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;_<br/>"
	resp += "(_)_ __ __ _ ___| |_<br/>"
	resp += "| | '__/ _` / __| '_ \\<br/>"
	resp += "| | |&nbsp;| (_| \\__ \\ | | |<br/>"
	resp += "|_|_|&nbsp;&nbsp;\\__, |___/_| |_|<br/>"
	resp += "&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;|___/<br/>"
	resp += "irgsh-chief " + app.Version
	resp += "<br/>"
	resp += "<br/><a href=\"/maintainers\">maintainers</a>&nbsp;|&nbsp;"
	resp += "<a href=\"/logs\">logs</a>&nbsp;|&nbsp;"
	resp += "<a href=\"/artifacts\">artifacts</a>&nbsp;|&nbsp;"
	resp += "<a target=\"_blank\" href=\"https://github.com/blankon/irgsh-go\">about</a>"
	resp += "</div>"
	fmt.Fprintf(w, resp)
}

func MaintainersHandler(w http.ResponseWriter, r *http.Request) {
	gnupgDir := "GNUPGHOME=" + irgshConfig.Chief.GnupgDir
	if irgshConfig.IsDev {
		gnupgDir = ""
	}

	cmdStr := gnupgDir + " gpg --list-key | tail -n +2"

	output, err := exec.Command("bash", "-c", cmdStr).Output()
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "500")
		return
	}
	fmt.Fprintf(w, string(output))
}

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "{\"version\":\""+app.Version+"\"}")
}
