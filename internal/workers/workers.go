package workers

import (
    "context"
    "log"

    zb "github.com/camunda-cloud/zeebe/clients/go/pkg/zbc"
)

// helper to spin up a Zeebe worker
func newWorker(client zb.Client, jobType string, handler func(context.Context, zb.JobClient, zb.Job)) {
    client.NewJobWorker().
        JobType(jobType).
        Handler(func(jobClient zb.JobClient, job zb.Job) {
            handler(context.Background(), jobClient, job)
        }).
        Open()
}

// RegisterAll spins up workers for each service task
func RegisterAll(client zb.Client) {
    newWorker(client, "save-record", func(ctx context.Context, jc zb.JobClient, job zb.Job) {
        log.Println("Saving record:", job.VariablesAsMap())
        jc.NewCompleteJobCommand().JobKey(job.Key).Send(ctx)
    })
    newWorker(client, "validate-data", func(ctx context.Context, jc zb.JobClient, job zb.Job) {
        log.Println("Validating data")
        jc.NewCompleteJobCommand().JobKey(job.Key).
            VariablesFromString(`{"valid":true}`).
            Send(ctx)
    })
    newWorker(client, "update-billing", func(ctx context.Context, jc zb.JobClient, job zb.Job) {
        log.Println("Updating billing")
        jc.NewCompleteJobCommand().JobKey(job.Key).Send(ctx)
    })
    newWorker(client, "notify-department", func(ctx context.Context, jc zb.JobClient, job zb.Job) {
        log.Println("Notifying department")
        jc.NewCompleteJobCommand().JobKey(job.Key).Send(ctx)
    })
}
