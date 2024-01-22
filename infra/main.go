package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ecs"
	"github.com/pulumi/pulumi-awsx/sdk/v2/go/awsx/ecr"
	ecrx "github.com/pulumi/pulumi-awsx/sdk/v2/go/awsx/ecr"
	ecsx "github.com/pulumi/pulumi-awsx/sdk/v2/go/awsx/ecs"
	lbx "github.com/pulumi/pulumi-awsx/sdk/v2/go/awsx/lb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		appContainerPort := 8080
		if param := cfg.GetInt("appContainerPort"); param != 0 {
			appContainerPort = param
		}
		cpu := 512
		if param := cfg.GetInt("cpu"); param != 0 {
			cpu = param
		}
		memory := 128
		if param := cfg.GetInt("memory"); param != 0 {
			memory = param
		}

		// create a new vpc with default configuration
		// vpc, err := ec2.NewVpc(ctx, "vpc", nil)
		// if err != nil {
		// 	return err
		// }

		// // create a new security group for our cluster
		// securityGroup, err := awsec2.NewSecurityGroup(ctx, "securityGroup", &awsec2.SecurityGroupArgs{
		// 	VpcId: vpc.VpcId,
		// 	Egress: awsec2.SecurityGroupEgressArray{
		// 		&awsec2.SecurityGroupEgressArgs{
		// 			FromPort: pulumi.Int(0),
		// 			ToPort:   pulumi.Int(0),
		// 			Protocol: pulumi.String("-1"),
		// 			CidrBlocks: pulumi.StringArray{
		// 				pulumi.String("0.0.0.0/0"),
		// 			},
		// 			Ipv6CidrBlocks: pulumi.StringArray{
		// 				pulumi.String("::/0"),
		// 			},
		// 		},
		// 	},
		// })
		// if err != nil {
		// 	return err
		// }

		// An ECS cluster to deploy into
		cluster, err := ecs.NewCluster(ctx, "go-fargate-cluster", nil)
		if err != nil {
			return err
		}

		// An ALB to serve the container endpoint to the internet
		loadbalancer, err := lbx.NewApplicationLoadBalancer(ctx, "go-fargate-lb", &lbx.ApplicationLoadBalancerArgs{
			DefaultTargetGroup: &lbx.TargetGroupArgs{
				// NOTE: when changing the application port, you need to change the target group port as well, which is the Port on which targets (ECS Task) receive traffic
				Port:     pulumi.Int(appContainerPort),
				Protocol: pulumi.String("HTTP"),
			},
		})
		if err != nil {
			return err
		}

		// An ECR repository to store our application's container image
		repo, err := ecrx.NewRepository(ctx, "go-fargate-repo", &ecrx.RepositoryArgs{
			ForceDelete: pulumi.Bool(true),
		})
		if err != nil {
			return err
		}

		// Build and publish our application's container image from ./app to the ECR repository
		// Linux tasks with the ARM64 architecture don't support the Fargate Spot capacity provider.
		// For the Fargate launch type, the following AWS Regions do not support 64-bit ARM workloads:
		// 		US East (N. Virginia), the use1-az3 Availability Zone
		image, err := ecrx.NewImage(ctx, "go-fargate-arm-image", &ecr.ImageArgs{
			RepositoryUrl: repo.Url,
			Context:       pulumi.String("../app"),
			Platform:      pulumi.String("linux/arm64"),
		})
		if err != nil {
			return err
		}

		// Deploy an ECS Service on Fargate to host the application container
		_, err = ecsx.NewFargateService(ctx, "go-fargate-service", &ecsx.FargateServiceArgs{
			Cluster: cluster.Arn,
			//  NOTE: you need either NeteorkConfiguration or AssignPublicIp defined
			// NetworkConfiguration: &ecs.ServiceNetworkConfigurationArgs{
			// 	Subnets: vpc.PrivateSubnetIds,
			// 	SecurityGroups: pulumi.StringArray{
			// 		securityGroup.ID(),
			// 	},
			// },
			AssignPublicIp: pulumi.Bool(true),
			TaskDefinitionArgs: &ecsx.FargateServiceTaskDefinitionArgs{
				RuntimePlatform: &ecs.TaskDefinitionRuntimePlatformArgs{
					// required for ARM64 tasks on Fargate
					// you cannot ONLY specify linux/arm in the image definition, you also need to define the runtime platform on the ECS service
					OperatingSystemFamily: pulumi.String("LINUX"),
					CpuArchitecture:       pulumi.String("ARM64"),
				},
				Container: &ecsx.TaskDefinitionContainerDefinitionArgs{
					Name:      pulumi.String("app"),
					Image:     image.ImageUri,
					Cpu:       pulumi.Int(cpu),
					Memory:    pulumi.Int(memory),
					Essential: pulumi.Bool(true),
					PortMappings: ecsx.TaskDefinitionPortMappingArray{
						&ecsx.TaskDefinitionPortMappingArgs{
							ContainerPort: pulumi.Int(appContainerPort),
							TargetGroup:   loadbalancer.DefaultTargetGroup,
						},
					},
				},
			},
		})
		if err != nil {
			return err
		}

		// The URL at which the container's HTTP endpoint will be available
		ctx.Export("url", pulumi.Sprintf("http://%s", loadbalancer.LoadBalancer.DnsName()))
		return nil
	})
}
