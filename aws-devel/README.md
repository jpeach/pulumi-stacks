# AWS Dev Environment

This Pulumi stack creates a development environment in AWS.

A new VPC is created, and a number of workload instances are provisioned inside
the VPC. The workload hosts can be accessed through the SSH bastion, using the
SSH key that is written to `./ssh-key`. That key is good for all the hosts.

All the hosts are provisioned with Fedora 34, so ssh login is as the `fedora`
user.

## Sample session

```
$ pulumi up
Previewing update (dev)

View Live: https://app.pulumi.com/jpeach/aws-devel/dev/previews/b40e8ce0-f94f-4aec-a411-5637512ef92a

     Type                              Name             Plan       
 +   pulumi:pulumi:Stack               aws-devel-dev    create     
 +   ├─ aws:ec2:Vpc                    vpc              create     
 +   ├─ aws:ec2:KeyPair                dev              create     
 +   ├─ aws:ec2:Subnet                 dmz              create     
 +   ├─ aws:ec2:InternetGateway        gw               create     
 +   ├─ aws:ec2:Subnet                 workload         create     
 +   ├─ aws:ec2:SecurityGroup          ingress/22       create     
 +   ├─ aws:ec2:RouteTable             routes           create     
 +   ├─ aws:ec2:NetworkInterface       priv/2           create     
 +   ├─ aws:ec2:NetworkInterface       priv/3           create     
 +   ├─ aws:ec2:NetworkInterface       priv/1           create     
 +   ├─ aws:ec2:NetworkInterface       priv/0           create     
 +   ├─ aws:ec2:NetworkInterface       priv/4           create     
 +   ├─ aws:ec2:NetworkInterface       priv/5           create     
 +   ├─ aws:ec2:RouteTableAssociation  subnet/dmz       create     
 +   ├─ aws:ec2:RouteTableAssociation  subnet/workload  create     
 +   ├─ aws:ec2:Instance               bastion/0        create     
 +   ├─ aws:ec2:Instance               instance/3       create     
 +   ├─ aws:ec2:Instance               instance/2       create     
 +   ├─ aws:ec2:Instance               instance/1       create     
 +   ├─ aws:ec2:Instance               instance/0       create     
 +   ├─ aws:ec2:Instance               instance/4       create     
 +   └─ aws:ec2:Instance               instance/5       create     
 
Resources:
    + 23 to create

Do you want to perform this update? yes
Updating (dev)

View Live: https://app.pulumi.com/jpeach/aws-devel/dev/updates/46

     Type                              Name             Status      
 +   pulumi:pulumi:Stack               aws-devel-dev    created     
 +   ├─ aws:ec2:KeyPair                dev              created     
 +   ├─ aws:ec2:Vpc                    vpc              created     
 +   ├─ aws:ec2:InternetGateway        gw               created     
 +   ├─ aws:ec2:Subnet                 workload         created     
 +   ├─ aws:ec2:Subnet                 dmz              created     
 +   ├─ aws:ec2:SecurityGroup          ingress/22       created     
 +   ├─ aws:ec2:RouteTable             routes           created     
 +   ├─ aws:ec2:NetworkInterface       priv/5           created     
 +   ├─ aws:ec2:NetworkInterface       priv/2           created     
 +   ├─ aws:ec2:NetworkInterface       priv/0           created     
 +   ├─ aws:ec2:NetworkInterface       priv/4           created     
 +   ├─ aws:ec2:NetworkInterface       priv/3           created     
 +   ├─ aws:ec2:NetworkInterface       priv/1           created     
 +   ├─ aws:ec2:RouteTableAssociation  subnet/workload  created     
 +   ├─ aws:ec2:Instance               bastion/0        created     
 +   ├─ aws:ec2:RouteTableAssociation  subnet/dmz       created     
 +   ├─ aws:ec2:Instance               instance/5       created     
 +   ├─ aws:ec2:Instance               instance/2       created     
 +   ├─ aws:ec2:Instance               instance/0       created     
 +   ├─ aws:ec2:Instance               instance/4       created     
 +   ├─ aws:ec2:Instance               instance/3       created     
 +   └─ aws:ec2:Instance               instance/1       created     
 
Outputs:
    bastion.addr   : "13.54.244.123"
    workload.addr.0: "172.16.2.6"
    workload.addr.1: "172.16.2.7"
    workload.addr.2: "172.16.2.8"
    workload.addr.3: "172.16.2.9"
    workload.addr.4: "172.16.2.10"
    workload.addr.5: "172.16.2.11"

Resources:
    + 23 created

Duration: 55s

jpeach@greenling:~/src/pulumi-stacks/aws-devel$ ssh -i ssh-key fedora@13.54.244.123 cat /etc/os-release
NAME=Fedora
VERSION="34 (Cloud Edition)"
ID=fedora
VERSION_ID=34
VERSION_CODENAME=""
PLATFORM_ID="platform:f34"
PRETTY_NAME="Fedora 34 (Cloud Edition)"
ANSI_COLOR="0;38;2;60;110;180"
LOGO=fedora-logo-icon
CPE_NAME="cpe:/o:fedoraproject:fedora:34"
HOME_URL="https://fedoraproject.org/"
DOCUMENTATION_URL="https://docs.fedoraproject.org/en-US/fedora/34/system-administrators-guide/"
SUPPORT_URL="https://fedoraproject.org/wiki/Communicating_and_getting_help"
BUG_REPORT_URL="https://bugzilla.redhat.com/"
REDHAT_BUGZILLA_PRODUCT="Fedora"
REDHAT_BUGZILLA_PRODUCT_VERSION=34
REDHAT_SUPPORT_PRODUCT="Fedora"
REDHAT_SUPPORT_PRODUCT_VERSION=34
PRIVACY_POLICY_URL="https://fedoraproject.org/wiki/Legal:PrivacyPolicy"
VARIANT="Cloud Edition"
VARIANT_ID=cloud
```
