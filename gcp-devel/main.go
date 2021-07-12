package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os/user"
	"strings"

	"github.com/pulumi/pulumi-gcp/sdk/v5/go/gcp/compute"
	"github.com/pulumi/pulumi-gcp/sdk/v5/go/gcp/container"
	"github.com/pulumi/pulumi-gcp/sdk/v5/go/gcp/secretmanager"
	"github.com/pulumi/pulumi-gcp/sdk/v5/go/gcp/serviceaccount"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"golang.org/x/crypto/ssh"
)

// DefaultNamePrefix is the default prefix for resource names.
var DefaultNamePrefix string

type ConfigClustersKubernetes struct {
	Channel string
	Version string
}

type ConfigClustersNodeConfig struct {
	MachineType string
	Preemptible bool
	OauthScopes []string
}

type ConfigClusters struct {
	Kubernetes    ConfigClustersKubernetes
	NetworkPolicy bool
	NodeLocations []string
	NodeConfig    ConfigClustersNodeConfig
	Names         []string
}

type Config struct {
	Clusters ConfigClusters
	Location string
}

// GenerateSSHKeys ...
func GenerateSSHKeys() (string, string, error) {
	// IMPORTANT: For GCP it has to be 3072 bit key, when tried to use 2048 or
	//            4096 it was not adding it to the authorized_keys file on
	//            cluster nodes, even if the key was present in a gcp console
	//            (in node's metadata)
	priv, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return "", "", err
	}

	b := pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(priv),
	}

	privBuffer := bytes.NewBuffer([]byte{})
	if err := pem.Encode(privBuffer, &b); err != nil {
		return "", "", err
	}

	public, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return "", "", err
	}
	// f, err := os.OpenFile(privKeyPath, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0400)
	// if err != nil {
	// 	return err
	// }
	//
	// defer f.Close()

	return privBuffer.String(), base64.StdEncoding.EncodeToString(public.Marshal()), nil
}

var (
	PrivateKeySecretName = "kuma-main-ssh-private-key"
	PublicKeySecretName  = "kuma-main-ssh-public-key"
)

func genName(value ...string) string {
	if DefaultNamePrefix == "" {
		return strings.Join(value, "-")
	}

	return strings.Join(append([]string{DefaultNamePrefix}, value...), "-")
}

func main() {
	u, err := user.Current()
	if err != nil {
		log.Fatalf("%s", err)
	}

	pulumi.Run(func(ctx *pulumi.Context) error {
		conf := config.New(ctx, "")
		var clustersCfg ConfigClusters
		conf.RequireObject("clusters", &clustersCfg)
		DefaultNamePrefix = fmt.Sprintf(
			"%s-%s",
			conf.Require("resourcePrefix"),
			u.Username,
		)
		cfg := Config{
			Clusters: clustersCfg,
			Location: conf.Require("location"),
		}

		var privateKey string
		var publicKey string

		privateKeySecret, err := secretmanager.LookupSecretVersion(ctx, &secretmanager.LookupSecretVersionArgs{
			Secret: PrivateKeySecretName,
		})
		if err != nil {
			if privateKey, publicKey, err = GenerateSSHKeys(); err != nil {
				return err
			}

			_ = ctx.Log.Info("No ssh keys in GCP Secret Manager", nil)
			_ = ctx.Log.Info("To create them:", nil)
			_ = ctx.Log.Info("###", nil)

			privateKey = strings.ReplaceAll(privateKey, "\n", "\\n")

			createPrivKey := fmt.Sprintf("printf -- '%s' | gcloud secrets create %s --data-file=-", privateKey, PrivateKeySecretName)
			createPubKey := fmt.Sprintf("printf -- '%s' | gcloud secrets create %s --data-file=-", publicKey, PublicKeySecretName)

			_ = ctx.Log.Info(createPrivKey, nil)
			_ = ctx.Log.Info("###", nil)
			_ = ctx.Log.Info(createPubKey, nil)

			return errors.New("cannot proceed without necessary ssh keys")
		} else {
			publicKeySecret, err := secretmanager.LookupSecretVersion(ctx, &secretmanager.LookupSecretVersionArgs{
				Secret: PublicKeySecretName,
			})
			if err != nil {
				return err
			}

			privateKey = privateKeySecret.SecretData
			publicKey = publicKeySecret.SecretData
		}

		sshKeys := []string{
			fmt.Sprintf("%s:ssh-rsa %s %[1]s", u.Username, publicKey),
		}

		svcAcc, err := serviceaccount.NewAccount(ctx, genName(), &serviceaccount.AccountArgs{
			AccountId:   pulumi.String(genName()),
			DisplayName: pulumi.String(fmt.Sprintf("Service Account used for testing Kuma by: %s", u.Username)),
		})
		if err != nil {
			return err
		}

		network, err := compute.NewNetwork(ctx, genName("network"), &compute.NetworkArgs{
			AutoCreateSubnetworks: pulumi.Bool(false),
		})
		if err != nil {
			return err
		}

		subnetwork, err := compute.NewSubnetwork(ctx, genName("subnet"), &compute.SubnetworkArgs{
			IpCidrRange: pulumi.String("10.2.0.0/16"),
			Network:     network.ID(),
			Region:      pulumi.String(cfg.Location),
		}, pulumi.Parent(network), pulumi.DeleteBeforeReplace(true))
		if err != nil {
			return err
		}

		_, err = compute.NewFirewall(ctx, genName("allow-ssh"), &compute.FirewallArgs{
			Network: network.Name,
			Allows: compute.FirewallAllowArray{
				&compute.FirewallAllowArgs{
					Protocol: pulumi.String("tcp"),
					Ports: pulumi.StringArray{
						pulumi.String("22"),
					},
				},
			},
			SourceRanges: pulumi.StringArray{
				pulumi.String("0.0.0.0/0"),
			},
		}, pulumi.Parent(network))
		if err != nil {
			return err
		}

		for _, name := range clustersCfg.Names {
			cluster, err := CreateCluster(ctx, &cfg, genName(name), network, sshKeys, svcAcc, subnetwork)
			if err != nil {
				return err
			}

			kubeconfig := GenKubeconfig(cluster).(pulumi.StringOutput)
			kubeconfigSecretName := genName(name, "kubeconfig")

			// Let's store kubeconfig in Secret Manager where we could have access to it
			secret, err := secretmanager.NewSecret(ctx, kubeconfigSecretName, &secretmanager.SecretArgs{
				SecretId: pulumi.String(kubeconfigSecretName),
				Replication: secretmanager.SecretReplicationArgs{
					Automatic: pulumi.Bool(true),
				},
			})
			if err != nil {
				return err
			}

			_, err = secretmanager.NewSecretVersion(ctx, kubeconfigSecretName, &secretmanager.SecretVersionArgs{
				Secret:     secret.Name,
				SecretData: kubeconfig,
			}, pulumi.DependsOn([]pulumi.Resource{cluster, secret}))
			if err != nil {
				return err
			}

			ctx.Export(kubeconfigSecretName, kubeconfig)
		}

		ctx.Export("service-account", svcAcc.AccountId)
		ctx.Export("private-key", pulumi.ToSecret(privateKey))
		ctx.Export("public-key", pulumi.ToSecret(publicKey))

		return nil
	})
}

func GenKubeconfig(cluster *container.Cluster) pulumi.Output {
	return pulumi.All(
		cluster.Name,
		cluster.Endpoint,
		cluster.MasterAuth,
		cluster.Project,
		cluster.Location,
	).ApplyT(func(values []interface{}) string {
		name := values[0].(string)
		endpoint := values[1].(string)
		masterAuth := values[2].(container.ClusterMasterAuth)
		project := values[3].(string)
		location := values[4].(string)
		context := fmt.Sprintf("%s_%s_%s", project, location, name)

		return fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %[1]s
    server: https://%[2]s
  name: %[3]s
contexts:
- context:
    cluster: %[3]s
    user: %[3]s
  name: %[3]s
current-context: %[3]s
kind: Config
preferences: {}
users:
- name: %[3]s
  user:
    auth-provider:
      config:
        cmd-args: config config-helper --format=json
        cmd-path: gcloud
        expiry-key: '{.credential.token_expiry}'
        token-key: '{.credential.access_token}'
      name: gcp
`, *masterAuth.ClusterCaCertificate, endpoint, context)
	})
}

func CreateCluster(
	ctx *pulumi.Context,
	cfg *Config,
	name string,
	network *compute.Network,
	sshKeys []string,
	svcAcc *serviceaccount.Account,
	subnetwork *compute.Subnetwork,
) (*container.Cluster, error) {
	addonsConfig := container.ClusterAddonsConfigArgs{}

	args := &container.ClusterArgs{
		InitialNodeCount: pulumi.Int(1),
		Location:         pulumi.String(cfg.Location),
		MinMasterVersion: pulumi.String(cfg.Clusters.Kubernetes.Version),
		Network:          network.SelfLink,
		NodeConfig: container.ClusterNodeConfigArgs{
			MachineType: pulumi.String(cfg.Clusters.NodeConfig.MachineType),
			Metadata: pulumi.StringMap{
				"ssh-keys":                 pulumi.String(strings.Join(sshKeys, "\n")),
				"disable-legacy-endpoints": pulumi.String("true"),
			},
			OauthScopes:    pulumi.ToStringArray(cfg.Clusters.NodeConfig.OauthScopes),
			Preemptible:    pulumi.Bool(cfg.Clusters.NodeConfig.Preemptible),
			ServiceAccount: svcAcc.Email,
		},
		NodeLocations: pulumi.ToStringArray(cfg.Clusters.NodeLocations),
		NodeVersion:   pulumi.String(cfg.Clusters.Kubernetes.Version),
		ReleaseChannel: container.ClusterReleaseChannelArgs{
			Channel: pulumi.String(strings.ToUpper(cfg.Clusters.Kubernetes.Channel)),
		},
		Subnetwork: subnetwork.ID(),
	}

	if cfg.Clusters.NetworkPolicy {
		args.NetworkPolicy = container.ClusterNetworkPolicyArgs{
			Enabled:  pulumi.Bool(true),
			Provider: pulumi.String("CALICO"),
		}

		addonsConfig.NetworkPolicyConfig = container.ClusterAddonsConfigNetworkPolicyConfigArgs{
			Disabled: pulumi.Bool(false),
		}
	}

	args.AddonsConfig = addonsConfig

	return container.NewCluster(ctx, name, args)
}
