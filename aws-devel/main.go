package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"log"
	"os/user"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"inet.af/netaddr"

	"github.com/jpeach/pulumi-stacks/pkg/keys"
)

// Networks defines the IP ranges for the networks we will build.
var Networks = map[string]netaddr.IPPrefix{
	"vpc":      netaddr.MustParseIPPrefix("172.16.0.0/16"), // Whole VPC.
	"dmz":      netaddr.MustParseIPPrefix("172.16.1.0/24"), // Ingress DMZ.
	"workload": netaddr.MustParseIPPrefix("172.16.2.0/24"), // Workloads.
}

// Fedora34 is the Fedora34 AMI image. Pre-configured user is "fedora".
const Fedora34 = "ami-0edc79a9bdc9f4eba"

// DefaultNamePrefix is the default prefix for resource names.
var DefaultNamePrefix string

// SecurityGroups ...
var SecurityGroups = map[string]*ec2.SecurityGroup{}

// NameTags ...
func NameTags(ctx *pulumi.Context, id ...string) pulumi.StringMap {
	parts := append([]string{DefaultNamePrefix, ctx.Stack()}, id...)
	return pulumi.StringMap{
		"Name": pulumi.String(strings.Join(parts, "-")),
	}
}

// FirstAllocatable returns the first allocatable address in the network
// prefix.  Skips the zero address, and the first 3 that AWS reserves in
// each subnet.
//
// See https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html
func FirstAllocatable(net netaddr.IPPrefix) (netaddr.IP, error) {
	r := net.Range()
	addr := r.From()
	skipped := 0

	for {
		if addr == r.To() {
			return netaddr.IP{}, fmt.Errorf("network %s exhausted", net)
		}

		addr = addr.Next()
		if addr.IsZero() {
			return netaddr.IP{}, fmt.Errorf("network %s exhausted", net)
		}

		skipped++
		if skipped > 4 {
			return addr, nil
		}
	}
}

// SecGroupBastion is a security group for bastion instances.
func SecGroupBastion(ctx *pulumi.Context, vpc *ec2.Vpc) (*ec2.SecurityGroup, error) {
	return ec2.NewSecurityGroup(ctx, "bastion",
		&ec2.SecurityGroupArgs{
			VpcId: vpc.ID(),
			Ingress: &ec2.SecurityGroupIngressArray{
				// Allow inbound SSH.
				&ec2.SecurityGroupIngressArgs{
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					FromPort: pulumi.Int(22),
					ToPort:   pulumi.Int(22),
					Protocol: pulumi.String("tcp"),
				},
			},
			Egress: &ec2.SecurityGroupEgressArray{
				// Allow any outbound.
				&ec2.SecurityGroupEgressArgs{
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					FromPort: pulumi.Int(0),
					ToPort:   pulumi.Int(0),
					Protocol: pulumi.String("-1"),
				},
			},
			Tags: NameTags(ctx, "sec", "bastion"),
		},
	)
}

// SecGroupWorkload is a security group for workload instances.
func SecGroupWorkload(ctx *pulumi.Context, vpc *ec2.Vpc) (*ec2.SecurityGroup, error) {
	return ec2.NewSecurityGroup(ctx, "workload",
		&ec2.SecurityGroupArgs{
			VpcId: vpc.ID(),
			// Allow any inbound.
			Ingress: &ec2.SecurityGroupIngressArray{
				&ec2.SecurityGroupIngressArgs{
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					FromPort: pulumi.Int(0),
					ToPort:   pulumi.Int(0),
					Protocol: pulumi.String("-1"),
				},
			},
			// Allow any outbound.
			Egress: &ec2.SecurityGroupEgressArray{
				&ec2.SecurityGroupEgressArgs{
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					FromPort: pulumi.Int(0),
					ToPort:   pulumi.Int(0),
					Protocol: pulumi.String("-1"),
				},
			},
			Tags: NameTags(ctx, "sec", "workload"),
		},
	)
}

// InitSecurityGroups ...
func InitSecurityGroups(ctx *pulumi.Context, vpc *ec2.Vpc) error {
	sec := map[string]func(ctx *pulumi.Context, vpc *ec2.Vpc) (*ec2.SecurityGroup, error){
		"Bastion":  SecGroupBastion,
		"Workload": SecGroupWorkload,
	}

	for n, f := range sec {
		grp, err := f(ctx, vpc)
		if err != nil {
			return err
		}

		SecurityGroups[n] = grp
	}

	return nil
}

// NewBastion ...
func NewBastion(
	ctx *pulumi.Context,
	vpc *ec2.Vpc,
	subnet *ec2.Subnet,
	keys *ec2.KeyPair,
) (*ec2.Instance, error) {
	return ec2.NewInstance(ctx, fmt.Sprintf("bastion/%d", 0), &ec2.InstanceArgs{
		Ami:                      pulumi.String(Fedora34),
		InstanceType:             pulumi.String("t2.micro"),
		KeyName:                  keys.KeyName,
		SubnetId:                 subnet.ID(),
		AssociatePublicIpAddress: pulumi.Bool(true),
		VpcSecurityGroupIds: pulumi.StringArray{
			SecurityGroups["Bastion"].ID().ToStringOutput(),
		},
		CreditSpecification: &ec2.InstanceCreditSpecificationArgs{
			CpuCredits: pulumi.String("unlimited"),
		},
		Tags: NameTags(ctx, "bastion"),
	})
}

func main() {
	u, err := user.Current()
	if err != nil {
		log.Fatalf("%s", err)
	}

	DefaultNamePrefix = u.Username

	sshKey, err := keys.NewPublicKey("./ssh-key")
	if err != nil {
		log.Fatalf("%s", err)
	}

	pulumi.Run(func(ctx *pulumi.Context) error {
		keys, err := ec2.NewKeyPair(ctx, "dev", &ec2.KeyPairArgs{
			PublicKey: pulumi.String(ssh.MarshalAuthorizedKey(sshKey)),
			Tags:      NameTags(ctx, "keys"),
		})
		if err != nil {
			return err
		}

		vpc, err := ec2.NewVpc(ctx, "vpc", &ec2.VpcArgs{
			CidrBlock:        pulumi.String(Networks["vpc"].String()),
			EnableDnsSupport: pulumi.Bool(true),
			Tags:             NameTags(ctx, "vpc"),
		})
		if err != nil {
			return err
		}

		err = InitSecurityGroups(ctx, vpc)
		if err != nil {
			return err
		}

		dmzSubnet, err := ec2.NewSubnet(ctx, "dmz", &ec2.SubnetArgs{
			VpcId:               vpc.ID(),
			CidrBlock:           pulumi.String(Networks["dmz"].String()),
			MapPublicIpOnLaunch: pulumi.Bool(true),
			Tags:                NameTags(ctx, "dmz"),
		})
		if err != nil {
			return err
		}

		gw, err := ec2.NewInternetGateway(ctx, "gw", &ec2.InternetGatewayArgs{
			VpcId: vpc.ID(),
			Tags:  NameTags(ctx, "gw"),
		}, pulumi.Parent(dmzSubnet))
		if err != nil {
			return err
		}

		gwRoutes, err := ec2.NewRouteTable(ctx, "routes/gw", &ec2.RouteTableArgs{
			VpcId: vpc.ID(),
			Routes: ec2.RouteTableRouteArray{
				&ec2.RouteTableRouteArgs{
					CidrBlock: pulumi.String("0.0.0.0/0"),
					GatewayId: gw.ID(),
				},
			},
			Tags: NameTags(ctx, "gw-routes"),
		}, pulumi.Parent(dmzSubnet))
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "gw/dmz", &ec2.RouteTableAssociationArgs{
			SubnetId:     dmzSubnet.ID(),
			RouteTableId: gwRoutes.ID(),
		}, pulumi.Parent(gwRoutes))
		if err != nil {
			return err
		}

		workloadSubnet, err := ec2.NewSubnet(ctx, "workload", &ec2.SubnetArgs{
			VpcId:     vpc.ID(),
			CidrBlock: pulumi.String(Networks["workload"].String()),
			Tags:      NameTags(ctx, "workload"),
		})
		if err != nil {
			return err
		}

		natEIP, err := ec2.NewEip(ctx, "eip/nat", &ec2.EipArgs{
			Vpc:  pulumi.Bool(true),
			Tags: NameTags(ctx, "nat-eip"),
		})
		if err != nil {
			return err
		}

		// The NAT gateway has to be in dmz subnet so it can use the
		// internet gateway there to get out.
		nat, err := ec2.NewNatGateway(ctx, "nat", &ec2.NatGatewayArgs{
			AllocationId:     natEIP.ID(),
			SubnetId:         dmzSubnet.ID(),
			ConnectivityType: pulumi.String("public"),
			Tags:             NameTags(ctx, "nat"),
		}, pulumi.Parent(workloadSubnet))
		if err != nil {
			return err
		}

		natRoutes, err := ec2.NewRouteTable(ctx, "routes/nat", &ec2.RouteTableArgs{
			VpcId: vpc.ID(),
			Routes: ec2.RouteTableRouteArray{
				&ec2.RouteTableRouteArgs{
					CidrBlock:    pulumi.String("0.0.0.0/0"),
					NatGatewayId: nat.ID(),
				},
			},
			Tags: NameTags(ctx, "nat-routes"),
		}, pulumi.Parent(workloadSubnet))
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "nat/workload", &ec2.RouteTableAssociationArgs{
			SubnetId:     workloadSubnet.ID(),
			RouteTableId: natRoutes.ID(),
		}, pulumi.Parent(natRoutes))
		if err != nil {
			return err
		}

		// Per the guide linked below, the routing table with the NAT
		// gateway should be the main table.
		//
		// https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Scenario2.html
		_, err = ec2.NewMainRouteTableAssociation(ctx, "main", &ec2.MainRouteTableAssociationArgs{
			VpcId:        vpc.ID(),
			RouteTableId: natRoutes.ID(),
		})
		if err != nil {
			return err
		}

		bastion, err := NewBastion(ctx, vpc, dmzSubnet, keys)
		if err != nil {
			return err
		}

		ctx.Export("bastion.addr", bastion.PublicIp)

		addr, err := FirstAllocatable(Networks["workload"])
		if err != nil {
			return err
		}

		// Config for workload instances.
		workloadConf := config.New(ctx, "workload")

		for i := 0; i < workloadConf.RequireInt("instanceCount"); i++ {
			addr = addr.Next()
			if addr.IsZero() {
				return fmt.Errorf("IP range %s exhausted", Networks["workload"].String())
			}

			iface, err := ec2.NewNetworkInterface(ctx, fmt.Sprintf("priv/%d", i),
				&ec2.NetworkInterfaceArgs{
					SubnetId: workloadSubnet.ID(),
					PrivateIps: pulumi.StringArray{
						pulumi.String(addr.String()),
					},
					SecurityGroups: pulumi.StringArray{
						SecurityGroups["Workload"].ID().ToStringOutput(),
					},
					Tags: NameTags(ctx, fmt.Sprintf("iface-%d", i)),
				}, pulumi.Parent(workloadSubnet))
			if err != nil {
				return err
			}

			instanceType := workloadConf.Require("instanceType")

			_, err = ec2.NewInstance(ctx, fmt.Sprintf("instance/%d", i), &ec2.InstanceArgs{
				Ami:          pulumi.String(Fedora34),
				InstanceType: pulumi.String(instanceType),
				KeyName:      keys.KeyName,
				NetworkInterfaces: ec2.InstanceNetworkInterfaceArray{
					&ec2.InstanceNetworkInterfaceArgs{
						NetworkInterfaceId: iface.ID(),
						DeviceIndex:        pulumi.Int(0),
					},
				},
				CreditSpecification: &ec2.InstanceCreditSpecificationArgs{
					CpuCredits: pulumi.String("unlimited"),
				},
				Tags: NameTags(ctx, fmt.Sprintf("workload-%d", i)),
			}, pulumi.Parent(iface))
			if err != nil {
				return err
			}

			ctx.Export(fmt.Sprintf("workload.addr.%d", i), pulumi.String(addr.String()))

		}

		return nil
	})
}
