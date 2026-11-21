// Package hostjobs declares the background work the app ships with: the jobs the composition root registers on the
// scheduler. Each job wraps a feature's own operation; the feature keeps its rules, and the schedule and budget live
// here, next to the job's id.
//
// Layer: jobs. The package imports features and jobs.
package hostjobs
