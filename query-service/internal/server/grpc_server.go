package server

import (
	"context"
	"log/slog"
	"regexp"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gen "github.com/OscarNunezU/distributed-job-processor/query-service/gen/job/v1"
	"github.com/OscarNunezU/distributed-job-processor/query-service/internal/repository"
)

var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type JobServer struct {
	gen.UnimplementedJobServiceServer
	repo *repository.PostgresJobRepository
	log  *slog.Logger
}

func NewJobServer(repo *repository.PostgresJobRepository, log *slog.Logger) *JobServer {
	return &JobServer{repo: repo, log: log}
}

func (s *JobServer) GetJob(ctx context.Context, req *gen.GetJobRequest) (*gen.JobResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if !uuidRegex.MatchString(req.Id) {
		return nil, status.Errorf(codes.NotFound, "job %s not found", req.Id)
	}

	job, err := s.repo.FindByID(ctx, req.Id)
	if err != nil {
		s.log.Error("findByID failed", "id", req.Id, "error", err)
		return nil, status.Errorf(codes.Internal, "internal error")
	}
	if job == nil {
		return nil, status.Errorf(codes.NotFound, "job %s not found", req.Id)
	}

	s.log.Info("GetJob", "id", req.Id)
	return toProto(job), nil
}

func (s *JobServer) ListJobs(ctx context.Context, req *gen.ListJobsRequest) (*gen.ListJobsResponse, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}

	jobs, total, err := s.repo.FindAll(ctx, limit, req.Offset)
	if err != nil {
		s.log.Error("findAll failed", "error", err)
		return nil, status.Errorf(codes.Internal, "internal error")
	}

	resp := &gen.ListJobsResponse{Total: total}
	for _, j := range jobs {
		resp.Jobs = append(resp.Jobs, toProto(j))
	}

	s.log.Info("ListJobs", "limit", limit, "offset", req.Offset, "returned", len(resp.Jobs))
	return resp, nil
}

func toProto(j *repository.Job) *gen.JobResponse {
	return &gen.JobResponse{
		Id:          j.ID,
		Type:        j.Type,
		Status:      j.Status,
		Attempts:    j.Attempts,
		MaxAttempts: j.MaxAttempts,
		CreatedAt:   j.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}
