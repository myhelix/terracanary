package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/myhelix/terracanary/canarrors"
	"github.com/myhelix/terracanary/stacks"
	"github.com/spf13/cobra"

	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	var region, cluster, taskDefinition, containerName string
	var timeout time.Duration

	var runCmd = &cobra.Command{
		Use:   "run --region <region> --cluster <cluster> --task-def <taskDefARN> -- <cmd>",
		Short: "Run an ECS task and wait for success",
		Long: `Runs a single ECS task that is expected to end, and watches for a successful exit code. Note that an ECS task definition can contain multiple containers, so you must specify the container name that will receive the specified command (overriding its normal docker CMD). If a log group is specified, logs for the task will be output in not-quite-realtime.

If the task runs, but provides a non-0 exit code, terracanary will pass through the task exit code; if you need to respond to specific task exit codes, make sure they don't overlap terracanary exit codes (see canarrors/errors.go).'`,
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdWords := args

			ecsSvc := ecs.New(stacks.AWSSession, &aws.Config{
				Region: &region,
			})

			if timeout != 0 {
				// Wait for deadline, then exit; brutal but simple
				go func() {
					time.Sleep(timeout)
					canarrors.Timeout.Details("after ", timeout).Exit()
				}()
			}

			// Look up the task definition, to see if we can infer some config from it.
			dtdo, err := ecsSvc.DescribeTaskDefinition(&ecs.DescribeTaskDefinitionInput{
				TaskDefinition: &taskDefinition,
			})
			canarrors.ExitIf(err)

			containers := dtdo.TaskDefinition.ContainerDefinitions
			var container *ecs.ContainerDefinition

			if containerName == "" {
				// If there's only one container, we can assume it's the one you want.
				if len(containers) != 1 {
					canarrors.ExitWith(fmt.Errorf("Cannot infer contain name from task definition; please specify with --container. Found container definitions:\n%v", containers))
				}
				container = containers[0]
				containerName = *container.Name
			} else {
				// Find our specified container
				for _, c := range containers {
					if *c.Name == containerName {
						container = c
						break
					}
				}
				if container == nil {
					canarrors.ExitWith(fmt.Errorf("Could not find container '%s' in task definition. Found container definitions:\n%v", containerName, containers))
				}
			}

			log.Println("Starting task with command: ", cmdWords)
			count := int64(1)
			// For future
			// Minimum Fargate size
			//cpu := int64(256)
			//memory := int64(512)
			rto, err := ecsSvc.RunTask(&ecs.RunTaskInput{
				Count:   &count,
				Cluster: &cluster,
				//LaunchType: aws.String("FARGATE"),
				TaskDefinition: &taskDefinition,
				Overrides: &ecs.TaskOverride{ContainerOverrides: []*ecs.ContainerOverride{{
					Name:    &containerName,
					Command: aws.StringSlice(cmdWords),
					//Cpu: &cpu,
					//Memory: &memory,
				}}},
			})
			canarrors.ExitIf(err)
			if len(rto.Failures) > 0 || len(rto.Tasks) != 1 {
				canarrors.ExitWith(fmt.Errorf("Error starting task:\n%v", rto))
			}
			task := rto.Tasks[0]
			log.Println("Started task:", *task.TaskArn)

			dti := ecs.DescribeTasksInput{
				Cluster: &cluster,
				Tasks:   []*string{task.TaskArn},
			}
			taskId := strings.Split(*task.TaskArn, "/")[1]
			logRelayer := NewLogRelayer(container, taskId)

			for {
				time.Sleep(1 * time.Second)

				// Start by relaying any new logs
				logRelayer()

				dto, err := ecsSvc.DescribeTasks(&dti)
				canarrors.ExitIf(err)
				if len(dto.Failures) > 0 || len(dto.Tasks) != 1 {
					if len(dto.Failures) == 1 && *dto.Failures[0].Reason == "MISSING" {
						log.Println("Task not ready; waiting for it to appear.")
						continue
					} else {
						canarrors.ExitWith(fmt.Errorf("Error checking on task:\n%v", rto))
					}
				}
				task = dto.Tasks[0]
				if *task.LastStatus == "STOPPED" {
					container := task.Containers[0]
					if container.ExitCode == nil {
						if container.Reason == nil {
							log.Printf("Task exited: %s\n", *task.StoppedReason)
						} else {
							log.Printf("Task container exited: %s\n", *container.Reason)
						}
						os.Exit(1)
					} else if code := *container.ExitCode; code != 0 {
						log.Printf("Task container exited with code %d\n", code)
						os.Exit(int(code))
					}
					log.Println("Task succeeded.")
					break
				}
			}
		},
	}
	runCmd.Flags().StringVar(&region, "region", "", "AWS region of cluster")
	runCmd.Flags().StringVar(&cluster, "cluster", "", "Name of ECS cluster")
	runCmd.Flags().StringVar(&taskDefinition, "task-def", "", "ECS task definition ARN")
	runCmd.Flags().StringVar(&containerName, "container", "", "Name of container to use inside task definition")
	runCmd.Flags().DurationVar(&timeout, "timeout", 0, "Timeout (default wait forever)")
	runCmd.MarkFlagRequired("region")
	runCmd.MarkFlagRequired("cluster")
	runCmd.MarkFlagRequired("task-def")
	ecsCmd.AddCommand(runCmd)
}

func NewLogRelayer(container *ecs.ContainerDefinition, taskId string) func() {
	if *container.LogConfiguration.LogDriver != "awslogs" {
		log.Println("Container not using awslogs; won't output logs.")
		return func() {}
	}
	getOpt := func(k string) string {
		v := container.LogConfiguration.Options[k]
		if v != nil {
			return *v
		}
		return ""
	}

	region := getOpt("awslogs-region")
	logGroup := getOpt("awslogs-group")
	logStreamPrefix := getOpt("awslogs-stream-prefix")

	if region == "" {
		log.Println("Could not find awslogs region; won't output logs.")
		return func() {}
	}

	if logGroup == "" {
		log.Println("Could not find awslogs group; won't output logs.")
		return func() {}
	}

	cwSvc := cloudwatchlogs.New(stacks.AWSSession, &aws.Config{
		Region: &region,
	})

	logStream := filepath.Join(logStreamPrefix, *container.Name, taskId)
	fmt.Println("Looking for logs at:", logStream)

	var logsFrom int64
	return func() {
		resp, err := cwSvc.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
			StartFromHead: aws.Bool(true),
			StartTime:     &logsFrom,
			LogGroupName:  &logGroup,
			LogStreamName: &logStream,
		})
		if err != nil {
			if awsErr, ok := err.(awserr.Error); ok &&
				awsErr.Code() == cloudwatchlogs.ErrCodeResourceNotFoundException {
				// This happens for a while every time while waiting for task to start; don't whine
			} else {
				log.Printf("Error getting logs: %s\n", err.Error())
			}
		} else {
			for _, event := range resp.Events {
				t := time.Unix(0, *event.Timestamp*int64(time.Millisecond))
				log.Println(t.String(), *event.Message)
				// Update cursor for next query; maybe not perfect but easy
				logsFrom = *event.Timestamp + 1
			}
		}
	}
}
