package main

import (
	"crypto/ed25519"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/crypto/ssh"

	"github.com/urfave/cli/v2"
)

type Keypair struct {
	// Unique name of this key pair.
	Id string

	// Ed25519 public key in OpenSSH format.
	PublicKey string

	// Ed25519 private key in OpenSSH format.
	PrivateKey string

	// GitHub repository URL.
	RepositoryURL string
}

type Keyring map[string]Keypair

func Load(file string) (Keyring, error) {
	// Read the full content of key ring file.
	data, err := os.ReadFile(file)
	if err != nil {
		// If the key ring does not exist, do not exit. Instead, return an empty
		// key ring. This is necessary to start a new key ring with "generate"
		// command.
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("Warning: key ring %q does not exist\n", file)
			return Keyring{}, nil
		}
		return nil, fmt.Errorf("failed to read key ring file: %w", err)
	}

	// Deserialize JSON to Keyring.
	keyring := Keyring{}
	if err := json.Unmarshal(data, &keyring); err != nil {
		return nil, fmt.Errorf("failed to read JSON from key ring file: %w", err)
	}

	return keyring, nil
}

func (kr Keyring) Save(file string) error {
	// Serialize Keyring to JSON.
	data, err := json.Marshal(kr)
	if err != nil {
		return fmt.Errorf("failed to conver to to JSON the key ring: %w", err)
	}

	// Write the encoded JSON to the key ring file. If doesn't exists, create it
	// with strict permissions.
	if err := os.WriteFile(file, data, 0666); err != nil {
		return fmt.Errorf("failed to save the key ring: %w", err)
	}

	return nil
}

// GenerateKeys returns an Ed25519 SSH key pair (public and private keys).
//
// The keys are encoded as string in OpenSSH format.
func GenerateKeys() (string, string, error) {
	// Generate a new ed25519 key pair.
	edPublic, edPrivate, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Errorf("Failed to generate ed25519 key pair: %w", err)
		return "", "", err
	}

	// Create a new SSH public key from the raw key.
	sshPublic, err := ssh.NewPublicKey(edPublic)
	if err != nil {
		fmt.Errorf("Failed to create SSH public key: %w", err)
		return "", "", err
	}

	// Serialize the SSH public key to OpenSSH format.
	strPublic := string(ssh.MarshalAuthorizedKey(sshPublic))

	// Serialize the SSH private key to OpenSSH format. It returns a PEM block.
	pemPrivate, err := ssh.MarshalPrivateKey(edPrivate, "")
	if err != nil {
		fmt.Errorf("Failed to create PEM block for private key: %w", err)
		return "", "", err
	}

	// Convert the PEM block to string.
	strPrivate := string(pem.EncodeToMemory(pemPrivate))

	return strPublic, strPrivate, nil
}

// CloneRepository clones the specified Git repository over SSH using the
// specified SSH key (in OpenSSH format) to the specified destination folder.
func CloneRepository(repositoryURL, privateKey, dest string) error {
	file := "key"

	// dest directory must be empty or not existent.
	entries, err := os.ReadDir(dest)
	if err != nil {
		return fmt.Errorf("failed to check %q directory: %w", dest, err)
	}
	if len(entries) > 0 {
		fmt.Fprintf(os.Stderr, "%q folder must be empty to clone the repository\n", dest)
		os.Exit(1)
	}

	// The privateKey must be temporarily saved to disk to be used by the SSH
	// client.
	//
	// The permission bit must be restricted, otherwise the SSH client won't be
	// able to read the key file.
	if err := os.WriteFile(file, []byte(privateKey), 0600); err != nil {
		return fmt.Errorf("failed to create temporary file for private key: %w", err)
	}
	defer func() { // Remember to delete the file at exit.
		if err := os.Remove(file); err != nil {
			fmt.Errorf("failed to delete remporary file for private key: %w", err)
		}
	}()

	// The command to execute.
	//
	// An complete example is:
	// 		git clone -c "core.sshCommand=ssh -i TMP_FILE -o IdentitiesOnly=yes"
	//			git@github.com:username/private-repo.git
	//
	// The core idea is to instruct the SSH client, called by Git, to use our
	// private key and not ask the SSH agent for other keys.
	cmd := exec.Command("git", "clone",
		"-c", fmt.Sprintf("core.sshCommand=ssh -i %s -o IdentitiesOnly=yes", file),
		repositoryURL, dest)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run git clone: %w", err)
	}

	return nil
}

// CliGenerateKeypair implements the "generate" command.
//
// It generates a new SSH key pair and saves the updated key ring to the disk.
// The command requires two arguments: id and repository URL. The ID must be
// an unique string in the key grip.
func CliGenerateKeypair(ctx *cli.Context) error {
	keyringFile := ctx.String("keyring")

	// Load the key ring from file.
	keyring, err := Load(keyringFile)
	if err != nil {
		return fmt.Errorf("failed to load key ring: %w", err)
	}

	// Extract arguments from command line.
	args := ctx.Args()
	if args.Len() != 2 {
		fmt.Fprintln(os.Stderr, "Missing mandatory arguments: [id] and [Repository URL].")
		os.Exit(1)
	}
	id := args.Get(0)
	repoURL := args.Get(1)

	// Do not overwrite an existing key pair.
	if _, exist := keyring[id]; exist {
		fmt.Fprintf(os.Stderr, "Key pair id %q already exists\n", id)
		os.Exit(1)
	}

	// Generate a SSH key pair.
	public, private, err := GenerateKeys()
	if err != nil {
		return fmt.Errorf("failed to generate key pair: %w", err)
	}

	keyring[id] = Keypair{
		Id:            id,
		PublicKey:     public,
		PrivateKey:    private,
		RepositoryURL: repoURL,
	}

	// The "generate" command modifies the key ring. It must be saved to disk.
	if err := keyring.Save(keyringFile); err != nil {
		return fmt.Errorf("failed to save key ring: %w", err)
	}

	// Just print at the end the public key. This will be sent to the repository
	// owner.
	fmt.Print(public)

	return nil
}

// CliCloneRepository implements the "clone" command.
//
// It clones the Git repository associated to a SSH key pair. This is why the
// command requires to specify the ID of the key pair.
func CliCloneRepository(ctx *cli.Context) error {
	keyringFile := ctx.String("keyring")

	// Load the key ring from file.
	keyring, err := Load(keyringFile)
	if err != nil {
		return fmt.Errorf("failed to load key ring: %w", err)
	}

	// Extract ID argument from command line.
	args := ctx.Args()
	if args.Len() < 1 {
		fmt.Fprintln(os.Stderr, "Missing mandatory [id] argument")
		os.Exit(1)
	} else if args.Len() > 1 {
		fmt.Fprintln(os.Stderr, "Too many arguments")
		os.Exit(1)
	}
	id := args.Get(0)

	// Get the selected key pair.
	keypair, exist := keyring[id]
	if !exist {
		fmt.Fprintf(os.Stderr, "Key pair %q does not exist\n", id)
		os.Exit(1)
	}

	if err := CloneRepository(keypair.RepositoryURL, keypair.PrivateKey, id); err != nil {
		return fmt.Errorf("failed to clone repository associated with id %q: %w", id, err)
	}

	return nil
}

// CliGetField prints the specified field of a key pair via its ID. If not ID is
// provided, it prints all saved IDs.
func CliGetField(ctx *cli.Context) error {
	keyringFile := ctx.String("keyring")

	// Load the key ring from file.
	keyring, err := Load(keyringFile)
	if err != nil {
		return fmt.Errorf("failed to load key ring: %w", err)
	}

	// Extract arguments from command line.
	args := ctx.Args()
	switch args.Len() {
	case 0:
		// Just print the saved IDs.
		for id := range keyring {
			fmt.Println(id)
		}
		return nil
	case 1:
		fmt.Fprintln(os.Stderr, "Missing field argument")
		os.Exit(1)
	case 2:
		// It is ok, normal flow after the switch.
		break
	default:
		fmt.Fprintln(os.Stderr, "Too many arguments")
		os.Exit(1)
	}
	id := args.Get(0)
	field := args.Get(1)

	// Get the selected key pair.
	keypair, exist := keyring[id]
	if !exist {
		fmt.Fprintf(os.Stderr, "Key pair %q does not exist\n", id)
		os.Exit(1)
	}

	// Get and print the field.
	switch field {
	// PublicKey and PrivateKey already contain a final newline.
	case "PublicKey":
		fmt.Print(keypair.PublicKey)
	case "PrivateKey":
		fmt.Print(keypair.PrivateKey)
	case "RepositoryURL":
		fmt.Println(keypair.RepositoryURL)
	default:
		fmt.Fprintf(os.Stderr, "Unrecognized field %q\n", field)
		os.Exit(1)
	}

	return nil
}

func main() {
	// Create and configure the application.
	app := cli.NewApp()
	app.Name = "gdkm"
	app.Usage = "A small program to manage SSH keys to be used as GitHub deploy keys"
	app.Action = func(*cli.Context) error {
		fmt.Printf("Hello World!\n")
		return nil
	}

	// "--keyring" global option.
	fileFlag := &cli.StringFlag{
		Name:      "keyring",
		Value:     "keyring.json",
		Usage:     "JSON file where the SSH keys are stored",
		TakesFile: true,
	}

	app.Flags = []cli.Flag{fileFlag}

	// "generate" command.
	genCommand := &cli.Command{
		Name:      "generate",
		Usage:     "Print the public key of a new SSH key pair in the key ring",
		Args:      true,
		ArgsUsage: " [id] [Repository URL]",
		Action:    CliGenerateKeypair,
	}

	// "get" command.
	getCommand := &cli.Command{
		Name:      "get",
		Usage:     "Get a single field of the key ring. If id is not specified, return all ids.",
		Args:      true,
		ArgsUsage: " [id] [PublicKey|PrivateKey|RepositoryURL]",
		Action:    CliGetField,
	}

	// "clone" command.
	cloneCommand := &cli.Command{
		Name:      "clone",
		Usage:     "Clone the repository associated with the given key pair",
		Args:      true,
		ArgsUsage: " [id]",
		Action:    CliCloneRepository,
	}

	// "pull" command.

	app.Commands = []*cli.Command{genCommand, getCommand, cloneCommand}

	// Run the appliction.
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error during execution: %s\n", err)
		os.Exit(1)
	}
}
