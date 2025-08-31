# GDKM

**GDKM** is a small program for managing a set of SSH keys used to clone Git repositories. The SSH keys are generated and stored in an unencrypted JSON file, then retrieved when the user needs to clone a repository. The cloned repository will also contain the settings for Git to use the private key for subsequent commands involving SSH, and the private key will be copied inside the repository.

The reason for **GDKM** is to facilitate the use of SSH keys as [deployment keys in GitHub](https://docs.github.com/en/authentication/connecting-to-github-with-ssh/managing-deploy-keys#deploy-keys). These keys need to be unique for each repository, so if the user has a lot of repositories, it can be burdensome to manually manage each one with the standard way. In fact, GDKM stands for *Github Deploy Keys Manager*.

Why did I create **GDKM**? Because I was an adjunct professor for my university in a course that required the students to create a final project for the exam. They could not submit the project as a ZIP file, but instead had to link to the project on GitHub. But the repository had to be private and I did not want to have access to it. So the solution was to use the deploy keys, one for each student project. This program helped me manage the keys and clone the repository to be ready to evaluate the project.

The git repository is available online on both [GitLab](https://gitlab.com/ema-pe/gdkm) and [GitHub](https://github.com/ema-pe/gdkm). However, GitHub is a read-only mirror of GitLab.

> [!WARNING]  
> As of 2025, I will no longer be using gdkm, and the code will no longer be updated.

## Installation

Just clone the repository and build the project:

```bash
$ git clone https://gitlab.com/ema-pe/gdkm.git
$ cd gdkm
$ make gdkm       # If you have GNU Make...
$ go build ./...  # ...otherwise use Go build directly.
```

Or you can simply use the Go install command:

```bash
$ go install -v gitlab.com/ema-pe/gdkm@latest
```

## Usage

Running `gdkm --help` (or `gdkm help`) will show the help message and all available commands. There is also documentation on [pkg.go.dev](https://pkg.go.dev/gitlab.com/ema-pe/gdkm), but it is not useful because it shows the source code documentation.

Two important concepts:

1. *Key pair*: is an SSH key pair associated with a Git repository (usually on GitHub) and an ID. The ID is unique and chosen by the user as an arbitrary string.

2. *Keyring*: is a collection of key pairs stored in a JSON file in an *unencrypted format*.

By default the keyring path is `keyring.json` in the current directory, but can be changed with the `--keyring` option.

There are three commands:

1. *`generate`* generates a new SSH key pair (using ED25519) and stores it in OpenSSH format as strings in the keyring. It also prints the public key to standard output. The user must provide two arguments: the ID of the new key pair and the URL of the associated git repository.

2. *`get`* returns one field of the key pair. If no ID is given, it prints all available IDs in the keyring. This command is for debugging and information purposes only.

3. *`clone`* clones the Git repository associated with the given ID using the associated SSH key pair.  If is a wrapper for the `git clone` command. The repository is cloned into the current directory and *the SSH secret key is copied into it*, allowing subsequent git commands to be used independently of GDKM. 

Example:

```bash
$ gdkm generate myID git@github.com:someUser/privateRepo.git
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA7nlMFJPYe3VaUnexWZ6PDEVcn3tn5ElcDWKBReOD29
$ gdkm get 
myID
$ gdkm get myID RepositoryURL
git@github.com:someUser/privateRepo.git
$ gdkm get myID PublicKey
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIA7nlMFJPYe3VaUnexWZ6PDEVcn3tn5ElcDWKBReOD29
$ gdkm clone myID  # Make sure the "someUser" added the deploy key in the project!
$ ls
keyring.json privateRepo
$ cd privateRepo
$ git pull  # Note that the git command uses the private key.
Already up to date.
$ ls
key ...  # "key" is the private key copied by GDKM.
```

## License

Copyright (c) 2024 Emanuele Petriglia

All right reserved. The project is licensed under the MIT License. See [`LICENSE`](LICENSE) file for more information.
