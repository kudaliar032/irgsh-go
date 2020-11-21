package repo

import (
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	model "github.com/blankon/irgsh-go/internal/artifact/model"

	tar "github.com/blankon/irgsh-go/pkg/tar"
)

// FileRepo interface with file system based artifact information
type FileRepo struct {
	Workdir string
}

// NewFileRepo create new instance
func NewFileRepo(Workdir string) *FileRepo {
	return &FileRepo{
		Workdir: Workdir,
	}
}

// GetArtifactList ...
// paging is not implemented yet
func (A *FileRepo) GetArtifactList(pageNum int64, rows int64) (artifactsList ArtifactList, err error) {
	files, err := filepath.Glob(A.Workdir + "/artifacts/*")
	if err != nil {
		return
	}

	artifactsList.Artifacts = []model.Artifact{}

	for _, file := range files {
		artifactsList.Artifacts = append(artifactsList.Artifacts, model.Artifact{Name: getArtifactFilename(file)})
	}
	artifactsList.TotalData = len(artifactsList.Artifacts)

	return
}

// PutTarballToFile not it's just general function to write string of base64 to file
func (A *FileRepo) PutTarballToFile(tarball *string, taskUUID string) (err error) {

	// create artifact directory
	submissionDir := A.generateSubmissionPath(taskUUID)
	err = os.MkdirAll(submissionDir, os.ModeDir)
	if err != nil {
		return
	}

	// write the tarball
	filePath := submissionDir + "/" + taskUUID + ".tar.gz"
	buff, err := base64.StdEncoding.DecodeString(*tarball)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(filePath, buff, 07440)
	if err != nil {
		return
	}

	return
}

// ExtractSubmittedTarball ...
func (A *FileRepo) ExtractSubmittedTarball(taskUUID string) (err error) {
	submissionDir := A.generateSubmissionPath(taskUUID)
	filePath := submissionDir + "/" + taskUUID + ".tar.gz"

	err = tar.ExtractTarball(filePath, submissionDir)

	return
}

func getArtifactFilename(filePath string) (fileName string) {
	path := strings.Split(filePath, "artifacts/")
	if len(path) > 1 {
		fileName = path[1]
	}
	return
}

func (A *FileRepo) generateSubmissionPath(taskUUID string) (path string) {
	path = A.Workdir + "/submissions/" + taskUUID
	return
}
