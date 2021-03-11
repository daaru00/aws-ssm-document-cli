## About this project

Right now the only methods available to deploy AWS SSM Documents are: 
- Using CloudFormation template that can deploy SSM document but cannot set a fixed name otherwise the deploy will fail.
- Using the web console (until the page is reloaded) but a little bit uncomfortable

This is a third method that allows you to version the code, deploy and manage documents.

PS: Also tried [Serverless Components](https://github.com/daaru00/serverless-plugin-ssm-document) but starts having issue with deployments.

## Install CLI

Download last archive package version from [releases page](https://github.com/daaru00/aws-ssm-document-cli/releases):

* Windows: aws-ssm-document_VERSION_Windows_x86_64.zip
* Mac: aws-ssm-document_VERSION_Darwin_x86_64.tar.gz
* Linux: aws-ssm-document_VERSION_Linux_x86_64.tar.gz

Unpack it and copy `aws-ssm-document` into one of your executable paths, for example, for Mac and Linux users:
```bash
tar -czvf aws-ssm-document_*.tar.gz
sudo mv aws-ssm-document /usr/local/bin/aws-ssm-document
rm aws-ssm-document_*.tar.gz
```

### For Linux Users

You can also install CLI from deb or rpm package downloading from releases page:

* aws-ssm-document_1.0.0_linux_amd64.deb
* aws-ssm-document_1.0.0_linux_amd64.rpm

### For Mac Users

Unlock CLI executable file going to "System Preference > Security and Privacy > General" and click on button "open anyway".

## Commands

Usage:
```bash
./aws-ssm-document [global options] command [command options] [path...]
```

- **deploy**: Deploy SSM Documents
- **remove**: Remove SSM Documents

## Environment configuration file

This CLI also load environment variable from `.env` file in current working directory:
```
AWS_PROFILE=my-profile
AWS_REGION=us-east-1
``` 

Setting `SSM_DOCUMENT_ENV` environment variable is it possible to load different env file:
```bash
export SSM_DOCUMENT_ENV=""
aws-ssm-document deploy # will load .env file
```
```bash
export SSM_DOCUMENT_ENV="prod"
aws-ssm-document deploy # will load .env.prod file
```
```bash
export SSM_DOCUMENT_ENV="STAGE"
aws-ssm-document deploy # will load .env.STAGE file
```

## Document configuration file

This CLI will search for `document.yml` configurations files, recursively, in search path (provided via first argument of any commands) for configurations file and deploy/remove documents in parallels. The document configuration file looks like this:
```yaml
name: MyDocument
description: My document description
accountIds: 
  - 123456789
file: ./script.sh
parameters:
  ParameterName:
    type: String
    description: "Parameter description"
  Command:
    type: String
    description: "Command to execute"
    default: "default-value"
tags:
  Project: my-project
  Environment: my-env
  Type: command
```

### Interpolation

In configuration file it is possible to interpolate environment variables using `${var}` or `$var` syntax:
```yaml
name: MyDocument
file: ./script.sh
tags:
  Project: "${APP_NAME}"
  Environment: "${ENV}"
```

### Search path

Any command accept file or directory paths as arguments, any document configuration file that match will be loaded an added to list.

If a directory is provided the CLI will search recursively for files `document.yml` (configurable via `--config-config-file`) 
and try to parse them using YAML parser (configurable via `--config-config-parser`), for example:
```bash
aws-ssm-document deploy ./documents
```

Search path and config file name can be set via environment variable (or `.env` file):
```
SSM_DOCUMENT_PATH=./documents/
SSM_DOCUMENT_CONFIG_FILE=*.yml
```

If a file is provided the CLI will be try to parse using YAML parser (configurable via `--config-config-parser`), for example:
```bash
aws-ssm-document deploy examples/simple/document.yml
```

Search path can be multiple, every argument respect the rules mentioned above:
```bash
aws-ssm-document deploy examples/simple/document.yml examples/parameters/document.yml examples/script/document.yml
# load 3 documents from provided files

aws-ssm-document deploy examples/ other-examples/script/document.yml
# load all documents in examples directory and a single one from other-examples

aws-ssm-document deploy examples/ other-examples/
# load all documents from examples and other-examples directories (all)
```

Also a file glob pattern can be used as search paths:
```bash
aws-ssm-document deploy examples/**/document.yml
# load 2 documents, one in nodejs directory and the other in the python one
```

Here an example of project configuration with single document:
```bash
.
└── my-command
    ├── document.yml
    └── script.sh
```

Here an example of project configuration with multiple documents:
```bash
.
└── documents
    ├── command1
    │   ├── document.yml
    │   └── script.sh
    ├── command2
    │   ├── document.yml
    │   └── script.sh
    └── command3
        ├── document.yml
        └── script.sh
```

Configuration file name can be changed via `--config-file` parameter:
```bash
aws-ssm-document deploy --config-file="ssm.yml" /commands/
.
└── commands
    ├── command1
    │   ├── ssm.yml
    │   └── script.sh
    ├── command2
    │   ├── ssm.yml
    │   └── script.sh
    └── command3
        ├── ssm.yml
        └── script.sh
```
both exact match and wildcard pattern can be used:
```bash
aws-ssm-document deploy --config-file="*.yml" /commands/
.
└── commands
    ├── command1.yml
    ├── command2.yml
    ├── command3.yml
    └── script.sh # export single script
```

## Deploy documents

To deploy documents run the `deploy` command:
```bash
aws-ssm-document deploy
```

## Remove documents

To remove (only) documents run the `remove` command:
```bash
aws-ssm-document remove
```
