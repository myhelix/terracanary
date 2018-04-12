package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/myhelix/terracanary/canarrors"
	"github.com/myhelix/terracanary/stacks"
	"github.com/spf13/cobra"

	"fmt"
	"log"
	"reflect"
	"sort"
	"time"
)

func init() {
	var region, cluster, service string
	var instances int64
	var timeout time.Duration

	var waitCmd = &cobra.Command{
		Use:   "wait --region <region> --cluster <cluster> (--instances <num> | --service <service>)",
		Short: "Wait for an ECS cluster to reach a stable state",
		Long: `Waits for an ECS cluster to reach an expected state.

If a number of instances is specified, wait until the cluster has that many instances joined to it.

If a service is specified, wait for it to reach the expected final state based upon its current configuration. This means:

* Task count of current task revision == desired tasks
* Task count of other task revisions == 0
* The instances backing the running tasks are in service with the service load balancer, if any`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if instances == -1 && service == "" {
				canarrors.ExitWith(fmt.Errorf("Must specify either --instances or --service."))
			}
			if timeout != 0 {
				// Wait for deadline, then exit; brutal but simple
				go func() {
					time.Sleep(timeout)
					canarrors.Timeout.Details("after ", timeout).Exit()
				}()
			}

			// If you really want to, you can do both.

			if instances != -1 {
				waitForInstances(region, cluster, instances)
			}

			if service != "" {
				waitForService(region, cluster, service)
			}

			log.Println("Done.")
		},
	}

	waitCmd.Flags().StringVar(&region, "region", "", "AWS region of cluster")
	waitCmd.Flags().StringVar(&cluster, "cluster", "", "Name of ECS cluster")
	waitCmd.Flags().Int64Var(&instances, "instances", -1, "Number of instances to wait for")
	waitCmd.Flags().StringVar(&service, "service", "", "Name of ECS service to wait for")
	waitCmd.Flags().DurationVar(&timeout, "timeout", time.Minute*10, "Timeout (default 10 min, 0 means forever)")
	waitCmd.MarkFlagRequired("region")
	waitCmd.MarkFlagRequired("cluster")
	ecsCmd.AddCommand(waitCmd)
}

func waitForInstances(region, cluster string, instances int64) {
	ecsSvc := ecs.New(stacks.AWSSession, &aws.Config{
		Region: &region,
	})
	var lastCount int64 = -1
	log.Printf("Waiting for instance count to be exactly %d for cluster %s", instances, cluster)
	for {
		dco, err := ecsSvc.DescribeClusters(&ecs.DescribeClustersInput{
			Clusters: []*string{&cluster},
		})
		canarrors.ExitIf(err)
		if len(dco.Clusters) != 1 || len(dco.Failures) > 0 {
			canarrors.ExitWith(fmt.Errorf("Error describing cluster: %v", dco))
		}
		count := *dco.Clusters[0].RegisteredContainerInstancesCount
		if count != lastCount {
			lastCount = count
			log.Printf("Instances: %d", count)
		}
		if count == instances {
			break
		}
		time.Sleep(time.Second * 3)
	}
}

func waitForService(region, cluster, service string) {
	ecsSvc := ecs.New(stacks.AWSSession, &aws.Config{
		Region: &region,
	})
	elbSvc1 := elb.New(stacks.AWSSession, &aws.Config{
		Region: &region,
	})
	elbSvc2 := elbv2.New(stacks.AWSSession, &aws.Config{
		Region: &region,
	})

	describeService := func() *ecs.Service {
		dso, err := ecsSvc.DescribeServices(&ecs.DescribeServicesInput{
			Services: []*string{&service},
			Cluster:  &cluster,
		})
		canarrors.ExitIf(err)
		if len(dso.Services) != 1 || len(dso.Failures) > 0 {
			canarrors.ExitWith(fmt.Errorf("Error describing service: %v", dso))
		}
		return dso.Services[0]
	}
	getTaskArns := func() (arns []string) {
		tasks, err := ecsSvc.ListTasks(&ecs.ListTasksInput{
			Cluster: &cluster,
		})
		canarrors.ExitIf(err)
		for _, task := range tasks.TaskArns {
			arns = append(arns, *task)
		}
		sort.Strings(arns)
		return arns
	}
	serv := describeService()
	waitForNumber := *serv.DesiredCount
	taskDefinition := *serv.TaskDefinition

	log.Printf("Waiting for count to be exactly %d of task %s", waitForNumber, taskDefinition)

	var lastOld, lastNew int64
	var finalTaskArns []string
	for {
		var newDep *ecs.Deployment
		var oldDepC, newDepC int64
		serv := describeService()
		for _, deployment := range serv.Deployments {
			if *deployment.TaskDefinition == taskDefinition {
				newDep = deployment
			} else {
				oldDepC += *deployment.RunningCount
			}
		}
		if newDep != nil {
			newDepC = *newDep.RunningCount
		}
		if oldDepC != lastOld || newDepC != lastNew {
			log.Printf("Old: %d\tCurrent: %d\n", oldDepC, newDepC)
			lastOld = oldDepC
			lastNew = newDepC
		}
		if oldDepC == 0 && newDepC == waitForNumber {
			// Check that our new situation is stable
			tasks1 := getTaskArns()
			time.Sleep(5 * time.Second)
			tasks2 := getTaskArns()

			// Recheck length here, in case the successful count was unstable, but we're now in a
			// stable bad state
			if int64(len(tasks1)) == waitForNumber && reflect.DeepEqual(tasks1, tasks2) {
				// True success
				finalTaskArns = tasks2
				break
			}
			log.Printf(`Found %d tasks, but task list is not stable. Tasks on first try:
%v
Second try:
%v
`, newDepC, tasks1, tasks2)
		} else {
			// No sign of success; keep waiting.
			time.Sleep(3 * time.Second)
		}
	}

	// Don't think there would currently ever be more than 1, but might as well use for
	for _, lb := range serv.LoadBalancers {
		// Need to describe tasks to determine what instances we expect to be in service
		dto, err := ecsSvc.DescribeTasks(&ecs.DescribeTasksInput{
			Cluster: &cluster,
			Tasks:   aws.StringSlice(finalTaskArns),
		})
		canarrors.ExitIf(err)
		if len(dto.Failures) > 0 {
			canarrors.ExitWith(fmt.Errorf("Error describing tasks: %v", dto))
		}

		var containerInstances []*string
		for _, task := range dto.Tasks {
			containerInstances = append(containerInstances, task.ContainerInstanceArn)
		}
		dcio, err := ecsSvc.DescribeContainerInstances(&ecs.DescribeContainerInstancesInput{
			Cluster:            &cluster,
			ContainerInstances: containerInstances,
		})
		canarrors.ExitIf(err)
		if len(dcio.Failures) > 0 {
			canarrors.ExitWith(fmt.Errorf("Error describing container instances: %v", dcio))
		}

		var name *string = lb.LoadBalancerName
		if name != nil {
			// v1 ELB
			log.Println("Waiting for targets to be in service with ELB: ", *name)

			var targets []*elb.Instance
			for _, ci := range dcio.ContainerInstances {
				targets = append(targets, &elb.Instance{
					InstanceId: ci.Ec2InstanceId,
				})
			}
			log.Println("Targets: ", targets)

			elbSvc1.WaitUntilInstanceInService(&elb.DescribeInstanceHealthInput{
				LoadBalancerName: name,
				Instances:        targets,
			})
		}

		var tgARN *string = lb.TargetGroupArn
		if tgARN != nil {
			// v2 ALB
			log.Println("Waiting for targets to be in service with ALB: ", *tgARN)

			var targets []*elbv2.TargetDescription
			for _, ci := range dcio.ContainerInstances {
				targets = append(targets, &elbv2.TargetDescription{
					Id: ci.Ec2InstanceId,
				})
			}
			log.Println("Targets: ", targets)

			elbSvc2.WaitUntilTargetInService(&elbv2.DescribeTargetHealthInput{
				TargetGroupArn: tgARN,
				Targets:        targets,
			})
		}
	}
}
