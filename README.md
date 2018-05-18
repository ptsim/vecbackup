# vecbackup

Versioned Encrypted Compressed backup.

* Backs up multiple versions locally
* De-duplicates based on content checksums (sha512_256)
* Compresses backups using gzip
* Optional symmetric encryption using AES-256
* Uses little memory and CPU. Works on Raspberry PI 256MB!
* Simple fast easy-to-understand single threaded implementation
* Only writes files to backup directory
* Suitable for remote backup by copying the backup directory using rsync etc
* MIT license.

** Release version 0.3 alpha **
** Not compatitble with earlier releases. **

**Use at your own risk.**

## How to use?

Suppose you want to back up ```/a/mystuff``` to ```/b/mybackup```.

First, initialize the backup directory:

```vecbackup init /b/mybackup```

Do the backup:

```vecbackup backup /a/mystuff /b/mybackup```

To see the timestamps of previous backups:

```vecbackup versions /b/mybackup```

To list the files in the backup:

```vecbackup files /b/mybackup```

To recover the latest backup to ```/a/temp```:

```vecbackup recover /b/mybackup /a/temp```

To test the recovery of the latest backup without writing the recovered files:

```vecbackup recover -t /b/mybackup /whatever```

To recover a file ```/a/mystuff/dir/something.doc``` from the backup to ```/a/temp/dir/something.doc```:

```vecbackup recover /b/mybackup /a/temp dir/something.doc```

To recover an older version of the same file:

```vecbackup recover -version <version_timestamp> /b/mybackup /a/temp dir/something.doc```

To verify that the backup files are not corrupted:

```vecbackup verify-backups /b/mybackup```

To delete old backup versions and reuse the space:

```vecbackup delete-old-versions /b/mybackup```

```vecbackup purge-old-data /b/mybackup```

## How to build?

* Install golang.
* Clone this repository.
* ```go build```
* ```go test``` (or ```go test -short```)

You will find the ```vecbackup``` binary in the current directory.

Tested with Golang 1.7 on OSX 10.11.6 and Raspian on Raspberry Pi Model B 256MB.

## Technical FAQ

### Q: How do I see all the options?
* Just run ```vecbackup``` and it will print all the commands and options.

### Q: How are files backed up?
* Each file is broken into 16MB chunks
* Each file is recorded as a list of chunks, metadata and whole file checksum.
* Each chunk is checksummed (sha512_256), compressed and encrypted (optional).
* Chunks are added and never modified or deleted during the backup operation
* A version manifest file (named with a RFC3339Nano timestamp) lists all the files for a version of the backup.

### Q: How does vecbackup know if files have been modified?
* vecbackup assumes that a file has not been modified if its file size and modified timestamp have not changed from the last backup.
* Use the ```backup -cs``` to checksum every file during backup. This is slow.

### Q: What about symbolic links, hard links, special files, empty directories and other special stuff?
* Symbolic links are backed up. It records the target location of the link.
* Hard links are backed up like normal files.
* Empty directories are backed up.
* Other special files are ignored silently.
* Unix permissions are recorded and recreated except the directories will be user writable.
* User and group ownership are ignored.
* Last modified timestamp for files are backed up.

### How do I automate or schedule my backups?
* I used crontab to run the backups automatically.
* When a backup is running, it maintains a ```vecbackup-lock``` file in the backup directory to prevent another instance from backing up to the same backup directory. This makes it easy to run timed backups without worrying about previous backups taking too long to complete.
* If a backup crashes for some reason, you may have to remove the ```vecbackup-lock``` file manually.
* The lock file is only used for the ```vecbackup backup``` command.

### Q: Can I have multiple "backup sets"?
* Yes, just backup different data to different backup directories.

### Q: How do I know if the files are recovered correctly?
* Each chunk has a sha512_256 checksum.
* Each file has a sha512_256 for the whole file.
* During recovery, the checksums are verified.
* Symbolic links and metadata like timestamps are not checksummed.
* Version manifest files are not checksummed.

### Q: Why are the chunks compressed?
* Because I have many uncompressed files.
* gzip is fast

### Q: What is the symmetric encryption for?
* So that I can clone the backup to "unsafe" remote or cloud storage or keep offline hard drives containing copies of the backup.

### Q: How do I use symmetric encrytion?
* Create a file containing your desired password
* Use ```-pw <path_to_your_password_file>``` for all commands. For example:

```vecbackup init -pw /a/mybkpw /b/mybackup```
* Save the password file separately from the actual backup directory.
* **```vecbackup init -pw``` generates and stores a random key in ```vecbackup-enc-config``` in the backup directory.**
* **Chunks and version files are encrypted with that key.**
* **```vecbackup-enc-config``` is encrypted with a key derived from your password using PBKDF2.**
* **If you lose the ```vecbackup-enc-config``` file, there is no way to recover the data.**

### Q: How do I tell vecbackup to skip (ignore) certain files?
* Create a file named ```vecbackup-ignore``` in the backup directory after running ```vecbackup init```.
* Each line in the file is a pattern containing files to ignore.
* Run ```vecbackup``` for more details.
* Example file:
``` 
.DS_Store
/a/abc/*
*~
```

### Q: Just show me the effects of the operations, aka dry run mode?
* ```vecbackup backup -n ...```
* ```vecbackup recover -n ...```
* ```vecbackup delete-old-versions -n ...```

### Q: Which older versions are kept for ```vecbackup delete-old-versions```?
* Keeps all versions within one day
* Keep one version per hour for the last week
* Keep one version per day in the last month
* Keep one version per week in the last year
* Keep one version per month otherwise
* All extra versions are deleted
* The unused chunk files are not deleted until you run ```vecbackup purge-old-data```.

### Q: Why don't you use XYZ software instead?
* Because various XYZ software have limitations that do not meet my requirements.

### Q: Is this ready for use?
* **This is an alpha release.**
* **Use at your own risk.**
* Having said that, I am using it actively **in conjunction** with other backup software.

### Q: What do you use this for?
* I use this to backup all my data, mostly consisting of terabytes of irreplaceable photos and videos.
