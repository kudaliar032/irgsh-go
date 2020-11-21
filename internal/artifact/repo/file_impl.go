package repo

import (
	"path/filepath"
	"strings"

	model "github.com/blankon/irgsh-go/internal/artifact/model"
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
func (A *FileRepo) PutTarballToFile(tarball *string) (err error) {
	return
}

func getArtifactFilename(filePath string) (fileName string) {
	path := strings.Split(filePath, "artifacts/")
	if len(path) > 1 {
		fileName = path[1]
	}
	return
}
