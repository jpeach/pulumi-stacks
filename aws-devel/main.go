package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"golang.org/x/crypto/ssh"
	"inet.af/netaddr"
)

// Ref https://www.pulumi.com/docs/reference/pkg/aws/ec2/instance/

// VPCPrefix is the CIDR network for the whole VPC.
var VPCPrefix = netaddr.MustParseIPPrefix("172.16.0.0/16")

// PublicSubnet is the CIDR network for the EC2 instances subnet.
var PublicSubnet = netaddr.MustParseIPPrefix("172.16.1.0/24")

// PrivateSubnet is the CIDR network for the EC2 instances subnet.
var PrivateSubnet = netaddr.MustParseIPPrefix("172.16.2.0/24")

// MaxInstances is the count of EC2 instances to build.
const MaxInstances = 1

// Fedora34 is the Fedora34 AMI image. Pre-configured user is "fedora".
const Fedora34 = "ami-0edc79a9bdc9f4eba"

// FirstAllocatable returns the first allocatable address in the network
// prefix.  Skips the zero address, and the first 3 that AWS reserves in
// each subnet.
//
// See https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Subnets.html
func FirstAllocatable(net *netaddr.IPPrefix) (netaddr.IP, error) {
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

// GeneratePrivateKey ...
func GeneratePrivateKey(privKeyPath string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	b := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}

	f, err := os.OpenFile(privKeyPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return err
	}

	defer f.Close()
	return pem.Encode(f, &b)
}

// FetchPrivateKey ...
func FetchPrivateKey(privKeyPath string) (ssh.PublicKey, error) {
	k, err := ioutil.ReadFile(privKeyPath)
	if os.IsNotExist(err) {
		if err := GeneratePrivateKey(privKeyPath); err != nil {
			return nil, err
		}

		k, err = ioutil.ReadFile(privKeyPath)
	}

	if err != nil {
		return nil, err
	}

	b, _ := pem.Decode(k)
	if b == nil {
		return nil, fmt.Errorf("no PEM data in %q", privKeyPath)
	}
	if b.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("wrong key type %q", b.Type)
	}

	priv, err := x509.ParsePKCS1PrivateKey(b.Bytes)
	if err != nil {
		return nil, err
	}

	return ssh.NewPublicKey(&priv.PublicKey)
}

func SecAllowIngressPort(
	ctx *pulumi.Context,
	vpc *ec2.Vpc,
	port int,
) (*ec2.SecurityGroup, error) {
	return ec2.NewSecurityGroup(ctx,
		fmt.Sprintf("ingress/%d", port),
		&ec2.SecurityGroupArgs{
			VpcId: vpc.ID(),
			Ingress: &ec2.SecurityGroupIngressArray{
				&ec2.SecurityGroupIngressArgs{
					CidrBlocks: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					FromPort: pulumi.Int(port),
					ToPort:   pulumi.Int(port),
					Protocol: pulumi.String("tcp"),
				},
			},
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
		},
	)
}

// NewBastion ...
func NewBastion(
	ctx *pulumi.Context,
	vpc *ec2.Vpc,
	subnet *ec2.Subnet,
	keys *ec2.KeyPair,
) (*ec2.Instance, error) {
	sec, err := SecAllowIngressPort(ctx, vpc, 22)
	if err != nil {
		return nil, err
	}

	return ec2.NewInstance(ctx, fmt.Sprintf("bastion/%d", 0), &ec2.InstanceArgs{
		Ami:                      pulumi.String(Fedora34),
		InstanceType:             pulumi.String("t2.micro"),
		KeyName:                  keys.KeyName,
		SubnetId:                 subnet.ID(),
		AssociatePublicIpAddress: pulumi.Bool(true),
		VpcSecurityGroupIds: pulumi.StringArray{
			sec.ID().ToStringOutput(),
		},

		CreditSpecification: &ec2.InstanceCreditSpecificationArgs{
			CpuCredits: pulumi.String("unlimited"),
		},
	})
}

func main() {
	sshKey, err := FetchPrivateKey("./ssh-key")
	if err != nil {
		log.Fatalf("%s", err)
	}

	pulumi.Run(func(ctx *pulumi.Context) error {
		keys, err := ec2.NewKeyPair(ctx, "dev", &ec2.KeyPairArgs{
			PublicKey: pulumi.String(ssh.MarshalAuthorizedKey(sshKey)),
			Tags:      pulumi.StringMap{},
		})
		if err != nil {
			return err
		}

		vpc, err := ec2.NewVpc(ctx, "vpc", &ec2.VpcArgs{
			CidrBlock:        pulumi.String(VPCPrefix.String()),
			EnableDnsSupport: pulumi.Bool(true),
			Tags:             pulumi.StringMap{},
		})
		if err != nil {
			return err
		}

		gw, err := ec2.NewInternetGateway(ctx, "gw", &ec2.InternetGatewayArgs{
			VpcId: vpc.ID(),
			Tags:  pulumi.StringMap{},
		})
		if err != nil {
			return err
		}

		public, err := ec2.NewSubnet(ctx, "public", &ec2.SubnetArgs{
			VpcId:               vpc.ID(),
			CidrBlock:           pulumi.String(PublicSubnet.String()),
			MapPublicIpOnLaunch: pulumi.Bool(true),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("jpeach/subnet/public"),
			},
		})
		if err != nil {
			return err
		}

		private, err := ec2.NewSubnet(ctx, "private", &ec2.SubnetArgs{
			VpcId:     vpc.ID(),
			CidrBlock: pulumi.String(PrivateSubnet.String()),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("jpeach/subnet/private"),
			},
		})
		if err != nil {
			return err
		}

		routes, err := ec2.NewRouteTable(ctx, "routes", &ec2.RouteTableArgs{
			VpcId: vpc.ID(),
			Routes: ec2.RouteTableRouteArray{
				&ec2.RouteTableRouteArgs{
					CidrBlock: pulumi.String("0.0.0.0/0"),
					GatewayId: gw.ID(),
				},
			},
			Tags: pulumi.StringMap{
				"Name": pulumi.String("jpeach/vpc/routes"),
			},
		})
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "association/private", &ec2.RouteTableAssociationArgs{
			SubnetId:     private.ID(),
			RouteTableId: routes.ID(),
		})
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "association/public", &ec2.RouteTableAssociationArgs{
			SubnetId:     public.ID(),
			RouteTableId: routes.ID(),
		})
		if err != nil {
			return err
		}

		bastion, err := NewBastion(ctx, vpc, public, keys)
		if err != nil {
			return err
		}

		ctx.Export("addr.bastion", bastion.PublicIp)

		addr, err := FirstAllocatable(&PrivateSubnet)
		if err != nil {
			return err
		}

		for i := 0; i < MaxInstances; i++ {
			iface, err := ec2.NewNetworkInterface(ctx, fmt.Sprintf("priv/%d", i),
				&ec2.NetworkInterfaceArgs{
					SubnetId: private.ID(),
					PrivateIps: pulumi.StringArray{
						pulumi.String(addr.String()),
					},
					Tags: pulumi.StringMap{},
				})
			if err != nil {
				return err
			}

			_, err = ec2.NewInstance(ctx, fmt.Sprintf("instance/%d", i), &ec2.InstanceArgs{
				Ami:          pulumi.String(Fedora34),
				InstanceType: pulumi.String("t2.micro"),
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
			})
			if err != nil {
				return err
			}

			ctx.Export(fmt.Sprintf("addr.instance.%d", i), pulumi.String(addr.String()))
		}

		return nil
	})
}
