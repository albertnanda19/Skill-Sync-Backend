package postgres

type JobRepository struct{}

func NewJobRepository() *JobRepository {
	return &JobRepository{}
}
