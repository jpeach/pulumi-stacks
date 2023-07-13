# Pulumi Stacks

This repository contains Pulimu stacks for setting up  development
environments on different cloud providers.

| Stack  | Description |
| --- | --- |
| [aws-devel](./aws-devel/README.md) | An AWS VPC with a configurable number of Fedora instances. |
| [gcp-devel](./gcp-devel/README.md) | A collection of GCP Kubernetes clusters. |

## How to use this

To get started, you need to
[install Pulumi](https://www.pulumi.com/docs/install/)
and create an account on the Pulumi cloud service.

Run `pulumi login` and follow the prompts to get an access token from the Pulumi cloud service.
I use my GitHub login to the Pulumi service.
You can use `pulumi whoami` to verify that you are logged in.

Then, you basically want to run `pulumi up` in the right directory. For example:
```bash
$ cd aws-devel
$ pulumi stack select dev
$ pulumi up
...
```

Pulumi will download plugins and start trying to bring up the resources.
For AWS, you will need to set the `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_SESSION_TOKEN` environment variables.
You can get these from the AWS login screen where it gives you the option to choose between
"Management console" or "Command line or programmatic access".

Running `pulumi stack` will show you the current resources in the stack.

It's likely that creating an EC2 instance on AWS will fail with an `OptInRequired` required error.
Like the error message says, you have to click on the AWS Marketplace link and accept the EULA
for the AMI image.
For reference, to get the AMI ID, you need to click from the Product page, through Subscribe to the Configure page, where
AWS will finally tell you what the ID is.
