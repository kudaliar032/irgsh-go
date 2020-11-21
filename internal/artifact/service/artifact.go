package service

import (
	"errors"
	"time"

	artifactModel "github.com/blankon/irgsh-go/internal/artifact/model"
	artifactRepo "github.com/blankon/irgsh-go/internal/artifact/repo"
	"github.com/google/uuid"
)

// ArtifactList list of artifact
type ArtifactList struct {
	TotalData int
	Artifacts []artifactModel.Artifact `json:"artifacts"`
}

// SubmissionJob job detail of the submission
type SubmissionJob struct {
	PipelineID string
	Jobs       []string
}

// Service interface for artifact service
type Service interface {
	GetArtifactList(pageNum int64, rows int64) (ArtifactList, error)
}

// ArtifactService implement service
type ArtifactService struct {
	repo artifactRepo.Repo
}

// NewArtifactService return artifact service instance
func NewArtifactService(repo artifactRepo.Repo) *ArtifactService {
	return &ArtifactService{
		repo: repo,
	}
}

// GetArtifactList get list of artifact
// paging is not yet functional
func (A *ArtifactService) GetArtifactList(pageNum int64, rows int64) (list ArtifactList, err error) {
	alist, err := A.repo.GetArtifactList(pageNum, rows)
	if err != nil {
		return
	}

	list.TotalData = alist.TotalData
	list.Artifacts = []artifactModel.Artifact{}

	for _, a := range alist.Artifacts {
		list.Artifacts = append(list.Artifacts, artifactModel.Artifact{Name: a.Name})
	}

	return
}

// SubmitPackage submit package
func (A *ArtifactService) SubmitPackage(tarball string) (job SubmissionJob, err error) {
	submittedJob := artifactModel.Submission{
		Timestamp: time.Now(),
	}

	submittedJob.TaskUUID = generateSubmissionUUID(submittedJob.Timestamp)

	err = A.repo.PutTarballToFile(&tarball, submittedJob.TaskUUID)
	if err != nil {
		return job, errors.New("Can't store tarball " + err.Error())
	}

	err = A.repo.ExtractSubmittedTarball(submittedJob.TaskUUID)
	if err != nil {
		return job, errors.New("Can't extract tarball " + err.Error())
	}

	// verify the package

	return
}

func generateSubmissionUUID(timestamp time.Time) string {
	return timestamp.Format("2006-01-02-150405") + "_" + uuid.New().String()
}
