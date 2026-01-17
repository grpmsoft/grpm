package daemon

import (
	"context"
	"fmt"
	"log"
	"time"

	pb "github.com/grpmsoft/grpm/api/gen"
	"github.com/grpmsoft/grpm/internal/application"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GRPMServer implements the gRPC service
// Following DDD pattern: this is a thin adapter layer that delegates to Application Service
type GRPMServer struct {
	pb.UnimplementedGRPMServiceServer
	daemon         *Daemon
	packageService *application.PackageService
}

// NewGRPMServer creates a new gRPC service instance
func NewGRPMServer(d *Daemon, pkgService *application.PackageService) *GRPMServer {
	return &GRPMServer{
		daemon:         d,
		packageService: pkgService,
	}
}

// RegisterGRPMService registers the gRPC service with daemon's gRPC server
func RegisterGRPMService(d *Daemon, pkgService *application.PackageService) {
	service := NewGRPMServer(d, pkgService)
	pb.RegisterGRPMServiceServer(d.grpcServer, service)
}

// Ping implements health check
func (s *GRPMServer) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingResponse, error) {
	return &pb.PingResponse{
		Message:   "pong",
		Timestamp: time.Now().Unix(),
	}, nil
}

// GetStatus returns daemon status
func (s *GRPMServer) GetStatus(ctx context.Context, req *pb.GetStatusRequest) (*pb.GetStatusResponse, error) {
	// Get daemon info from repository
	info, err := s.daemon.repository.GetInfo()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get daemon info: %v", err)
	}

	// Build response
	resp := &pb.GetStatusResponse{
		Daemon: &pb.DaemonStatus{
			State:         info.Status.String(),
			Pid:           int32(info.PID),
			UptimeSeconds: int64(info.Uptime.Seconds()),
			Version:       info.Version,
		},
		System: &pb.SystemStatus{
			InstalledPackages: 0,         // TODO: Query from VarDB
			AvailableUpdates:  0,         // TODO: Query from resolver
			PortageVersion:    "unknown", // TODO: Read from profile
		},
	}

	return resp, nil
}

// InstallPackage installs a package (streaming progress) using Job Queue
func (s *GRPMServer) InstallPackage(req *pb.InstallPackageRequest, stream pb.GRPMService_InstallPackageServer) error {
	log.Printf("[gRPC] InstallPackage called: %s", req.PackageName)

	// Create a job for installation
	job := NewJob(JobTypeInstall, req.PackageName, JobPriorityNormal)

	// Create progress channel for streaming to client
	progressChan := make(chan *pb.InstallPackageResponse, 10)

	// Set up progress callback
	job.progressCallback = func(progress int, message string) {
		progressChan <- &pb.InstallPackageResponse{
			JobId: job.ID,
			Event: &pb.InstallPackageResponse_Progress{
				Progress: &pb.ProgressEvent{
					Stage:     "installing",
					Message:   message,
					Percent:   int32(progress),
					Timestamp: time.Now().Unix(),
				},
			},
		}
	}

	// Define job execution function
	job.execute = func(ctx context.Context, j *Job) error {
		// Create progress channel for Application Service
		appProgressChan := make(chan application.InstallProgress, 10)

		// Start installation in goroutine
		errChan := make(chan error, 1)
		go func() {
			err := s.packageService.InstallPackage(req.PackageName, appProgressChan)
			close(appProgressChan)
			errChan <- err
		}()

		// Forward progress from Application Service to gRPC stream
		for progress := range appProgressChan {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				progressChan <- &pb.InstallPackageResponse{
					JobId: job.ID,
					Event: &pb.InstallPackageResponse_Progress{
						Progress: &pb.ProgressEvent{
							Stage:     progress.Stage,
							Message:   progress.Message,
							Percent:   int32(progress.Percent),
							Timestamp: progress.Timestamp,
						},
					},
				}
				j.UpdateProgress(progress.Percent, progress.Message)
			}
		}

		// Check for installation error
		return <-errChan
	}

	// Submit job to queue
	if err := s.daemon.jobQueue.Submit(job); err != nil {
		return status.Errorf(codes.ResourceExhausted, "failed to submit job: %v", err)
	}

	// Send initial response with job_id
	if err := stream.Send(&pb.InstallPackageResponse{
		JobId: job.ID,
		Event: &pb.InstallPackageResponse_Progress{
			Progress: &pb.ProgressEvent{
				Stage:     "queued",
				Message:   fmt.Sprintf("Job %s queued for package %s", job.ID, req.PackageName),
				Percent:   0,
				Timestamp: time.Now().Unix(),
			},
		},
	}); err != nil {
		return status.Errorf(codes.Internal, "failed to send initial response: %v", err)
	}

	// Stream progress events until job completes
	// We need to monitor both the progress channel and job status
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case resp, ok := <-progressChan:
			if ok {
				if err := stream.Send(resp); err != nil {
					return status.Errorf(codes.Internal, "failed to send progress: %v", err)
				}
			}

		case <-ticker.C:
			// Check if job is complete
			currentJob, err := s.daemon.jobQueue.GetJob(job.ID)
			if err != nil {
				return status.Errorf(codes.Internal, "failed to get job status: %v", err)
			}

			switch currentJob.GetStatus() {
			case JobStatusCompleted:
				close(progressChan)
				// Send completion event
				return stream.Send(&pb.InstallPackageResponse{
					JobId: job.ID,
					Event: &pb.InstallPackageResponse_Completion{
						Completion: &pb.CompletionEvent{
							Success:           true,
							Message:           fmt.Sprintf("Successfully installed %s", req.PackageName),
							InstalledPackages: []string{req.PackageName},
							Timestamp:         time.Now().Unix(),
						},
					},
				})

			case JobStatusFailed:
				close(progressChan)
				// Send error event
				return stream.Send(&pb.InstallPackageResponse{
					JobId: job.ID,
					Event: &pb.InstallPackageResponse_Error{
						Error: &pb.ErrorEvent{
							Error:     currentJob.GetError(),
							Details:   fmt.Sprintf("Installation failed for %s", req.PackageName),
							Timestamp: time.Now().Unix(),
						},
					},
				})

			case JobStatusCancelled:
				close(progressChan)
				// Send error event
				return stream.Send(&pb.InstallPackageResponse{
					JobId: job.ID,
					Event: &pb.InstallPackageResponse_Error{
						Error: &pb.ErrorEvent{
							Error:     "job canceled",
							Details:   fmt.Sprintf("Installation canceled for %s", req.PackageName),
							Timestamp: time.Now().Unix(),
						},
					},
				})
			}
		}
	}
}

// RemovePackage removes a package (streaming progress)
func (s *GRPMServer) RemovePackage(req *pb.RemovePackageRequest, stream pb.GRPMService_RemovePackageServer) error {
	// Stub: daemon API for removal not yet implemented
	// Use CLI standalone mode: grpm remove <package>
	return stream.Send(&pb.RemovePackageResponse{
		Event: &pb.RemovePackageResponse_Error{
			Error: &pb.ErrorEvent{
				Error:     "not implemented",
				Details:   "Package removal via daemon not yet implemented. Use CLI directly.",
				Timestamp: time.Now().Unix(),
			},
		},
	})
}

// UpdateSystem updates the system (streaming progress)
func (s *GRPMServer) UpdateSystem(req *pb.UpdateSystemRequest, stream pb.GRPMService_UpdateSystemServer) error {
	// Stub: daemon API for system update not yet implemented
	// Use CLI standalone mode: grpm update
	return stream.Send(&pb.UpdateSystemResponse{
		Event: &pb.UpdateSystemResponse_Error{
			Error: &pb.ErrorEvent{
				Error:     "not implemented",
				Details:   "System update via daemon not yet implemented. Use CLI directly.",
				Timestamp: time.Now().Unix(),
			},
		},
	})
}

// SearchPackages searches for packages
func (s *GRPMServer) SearchPackages(ctx context.Context, req *pb.SearchPackagesRequest) (*pb.SearchPackagesResponse, error) {
	log.Printf("[gRPC] SearchPackages called: query=%s, limit=%d", req.Query, req.Limit)

	// Delegate to Application Service
	result, err := s.packageService.SearchPackages(req.Query, int(req.Limit))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search failed: %v", err)
	}

	// Convert DTOs to protobuf messages
	var pbPackages []*pb.PackageInfo
	for _, pkg := range result.Packages {
		pbPackages = append(pbPackages, &pb.PackageInfo{
			Name:        pkg.Name,
			Version:     pkg.Version,
			Slot:        pkg.Slot,
			Description: pkg.Description,
			UseFlags:    pkg.UseFlags,
		})
	}

	return &pb.SearchPackagesResponse{
		Packages:   pbPackages,
		TotalCount: int32(result.TotalCount),
	}, nil
}

// GetPackageInfo gets information about a package
func (s *GRPMServer) GetPackageInfo(ctx context.Context, req *pb.GetPackageInfoRequest) (*pb.GetPackageInfoResponse, error) {
	log.Printf("[gRPC] GetPackageInfo called: %s", req.PackageName)

	// Delegate to Application Service
	info, err := s.packageService.GetPackageInfo(req.PackageName)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "package not found: %v", err)
	}

	// Convert DTO to protobuf message
	return &pb.GetPackageInfoResponse{
		Package: &pb.PackageInfo{
			Name:        info.Name,
			Version:     info.Version,
			Slot:        info.Slot,
			Description: info.Description,
			UseFlags:    info.UseFlags,
		},
		Details: &pb.PackageDetails{
			Homepage:     info.Homepage,
			License:      info.License,
			Dependencies: info.Dependencies,
		},
	}, nil
}

// ResolvePackage resolves package dependencies
func (s *GRPMServer) ResolvePackage(ctx context.Context, req *pb.ResolvePackageRequest) (*pb.ResolvePackageResponse, error) {
	log.Printf("[gRPC] ResolvePackage called: %s", req.PackageName)

	// Delegate to Application Service (single package for now)
	result, err := s.packageService.ResolvePackage([]string{req.PackageName})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "resolution failed: %v", err)
	}

	// Convert DTO to protobuf message
	packagesToInstall := make([]string, 0, len(result.PackagesToInstall))
	for name, version := range result.PackagesToInstall {
		packagesToInstall = append(packagesToInstall, fmt.Sprintf("%s-%s", name, version))
	}

	return &pb.ResolvePackageResponse{
		Success:           result.Success,
		PackagesToInstall: packagesToInstall,
		PackagesToUpdate:  result.PackagesToUpdate,
		Conflicts:         result.Conflicts,
		Error:             result.Error,
	}, nil
}

// GetJobStatus returns the status of a job
func (s *GRPMServer) GetJobStatus(ctx context.Context, req *pb.GetJobStatusRequest) (*pb.GetJobStatusResponse, error) {
	log.Printf("[gRPC] GetJobStatus called: job_id=%s", req.JobId)

	// Get job from queue
	job, err := s.daemon.jobQueue.GetJob(req.JobId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job not found: %v", err)
	}

	// Convert Job to JobInfo protobuf
	jobInfo := s.convertJobToProto(job)

	return &pb.GetJobStatusResponse{
		Job: jobInfo,
	}, nil
}

// ListJobs returns all jobs (optionally filtered by status)
func (s *GRPMServer) ListJobs(ctx context.Context, req *pb.ListJobsRequest) (*pb.ListJobsResponse, error) {
	log.Printf("[gRPC] ListJobs called: filter=%s", req.StatusFilter)

	// Get all jobs from queue
	allJobs := s.daemon.jobQueue.ListJobs()

	// Filter by status if requested
	var filteredJobs []*Job
	if req.StatusFilter != "" {
		for _, job := range allJobs {
			if job.GetStatus().String() == req.StatusFilter {
				filteredJobs = append(filteredJobs, job)
			}
		}
	} else {
		filteredJobs = allJobs
	}

	// Convert to protobuf
	var pbJobs []*pb.JobInfo
	for _, job := range filteredJobs {
		pbJobs = append(pbJobs, s.convertJobToProto(job))
	}

	return &pb.ListJobsResponse{
		Jobs:       pbJobs,
		TotalCount: int32(len(pbJobs)),
	}, nil
}

// CancelJob cancels a running or pending job
func (s *GRPMServer) CancelJob(ctx context.Context, req *pb.CancelJobRequest) (*pb.CancelJobResponse, error) {
	log.Printf("[gRPC] CancelJob called: job_id=%s", req.JobId)

	// Cancel job in queue
	err := s.daemon.jobQueue.CancelJob(req.JobId)
	if err != nil {
		return &pb.CancelJobResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to cancel job: %v", err),
		}, nil
	}

	return &pb.CancelJobResponse{
		Success: true,
		Message: fmt.Sprintf("Job %s canceled successfully", req.JobId),
	}, nil
}

// convertJobToProto converts internal Job to protobuf JobInfo
func (s *GRPMServer) convertJobToProto(job *Job) *pb.JobInfo {
	jobInfo := &pb.JobInfo{
		Id:          job.ID,
		Type:        string(job.Type),
		PackageName: job.PackageName,
		Status:      job.GetStatus().String(),
		Progress:    int32(job.GetProgress()),
		Error:       job.GetError(),
		CreatedAt:   job.CreatedAt.Unix(),
	}

	// Add timestamps if available
	if startedAt := job.GetStartedAt(); startedAt != nil {
		jobInfo.StartedAt = startedAt.Unix()
	}
	if completedAt := job.GetCompletedAt(); completedAt != nil {
		jobInfo.CompletedAt = completedAt.Unix()
	}

	return jobInfo
}
