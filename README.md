# vecbackup

Versioned Encrypted Compressed backup.

* Backs up multiple versions locally
* De-duplicates based on content checksums (sha512_256)
* Compresses backups (gzip)
* Optionally password protect and encrypt backups with authenticated encryption (PBKDF2+NaCl)
* MIT license.

**Disclaimer: Use at your own risk.**

## How to use?

Suppose you want to back up ```/a/mystuff``` to ```/b/mybackup```.

First, initialize the backup directory:

```vecbackup init /b/mybackup```

Do the backup:

```vecbackup backup /b/mybackup /a/mystuff```

To see the timestamps of previous backups:

```vecbackup versions /b/mybackup```

To list the files in the backup:

```vecbackup ls /b/mybackup```

To recover the latest backup to ```/a/temp```:

```vecbackup recover /b/mybackup /a/temp```

To test the recovery of the latest backup without writing the recovered files:

```vecbackup recover -t /b/mybackup /whatever```

To recover a file ```/a/mystuff/dir/something.doc``` from the backup to ```/a/temp/dir/something.doc```:

```vecbackup recover /b/mybackup /a/temp dir/something.doc```

To recover an older version of the same file:

```vecbackup recover -version <version_timestamp> /b/mybackup /a/temp dir/something.doc```

To verify that the all backup files of all versions are not corrupted:

```vecbackup verifybackups /b/mybackup```

To delete old backup versions and reuse the space:

```vecbackup deleteoldversions /b/mybackup```

```vecbackup purgeunused /b/mybackup```

## How to install?

Download the latest OS X or Linux release here:
https://github.com/ptsim/vecbackup/releases

For other systems, try building from source.
It will likely just work with any Linux distribution.

Not tested on Windows.

## How to build?

* Install golang.
* ```go get github.com/ptsim/vecbackup```
* ```go build```
* ```go test``` (or ```go test -longtest```)

You will find the ```vecbackup``` binary in the current directory.

The latest version was built and tested with Golang 1.11.5 on OSX 10.14.

## FAQ

### Q: How do I see all the options?
* Run ```vecbackup``` to print all the commands and options.
* Run ```vecbackup help``` for more detailed description of all the commands.

### Q: How are files backed up?
* Each file is broken into 16MB chunks. The size can be set with -chunksize flag during initialization.
* Each file is recorded as a list of chunks, metadata and whole file checksum.
* Each chunk is checksummed (sha512_256), compressed and optionally encrypted using Golang secretbox (NaCl).
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
* Unix permissions are recorded and recreated except that the directories will be user writable.
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
* If encryption is used, all chunks, symbolic links, metadata and version manifest files are encrypted using authenticated encryption (See NaCl).
* If no encryption is used, symbolic links, metadata and version manifest files are not checksummed.

### Q: Why are the chunks compressed?
* Because I have many uncompressed files.
* gzip is fast

### Q: How do I use encryption?
* Create a file containing your desired password
* Use ```-pw <path_to_your_password_file>``` for all commands. For example:

```vecbackup init -pw /a/mybkpw /b/mybackup```
* Use the ```-pbkdf2iterations <num>``` flag for the init command to set how slow key generation and key verification is. The larger the number, the slower it is. Default and minimum 100,000.
* If you lose your password, there is almost no way to recover the data in the backup.

### Q: What is the encryption for?
* So that I can copy the backups to "unsafe" remote, cloud or offline storage.
* With authenticated encryption, I can be sure the backup files have not been modified accidentially or intentionally.

### Q: Did you roll your own encryption scheme?
* No.
* The 256-bit master encryption key is derived from the user's password using PBKDF2.
* The 256-bit storage encryption key is randomly generated and encrypted using the master encryption key.
* All encrypted data is stored using Golang's secretbox module.
* Secretbox provides authenticated encryption and is interoperable with NaCl (https://nacl.cr.yp.to/).

## Q: How do I tell vecbackup to skip (ignore) certain files?
* Create a file named ```vecbackup-ignore``` in the backup directory after running ```vecbackup init```.
* Each line in the file is a pattern containing files to ignore.
* Run ```vecbackup help``` for more details.
* Example file:
``` 
.DS_Store
/a/abc/*
*~
```

### Q: Just show me the effects of the operations, aka dry run mode?
* ```vecbackup backup -n ...```
* ```vecbackup recover -n ...```
* ```vecbackup deleteoldversions -n ...```

### Q: Which older versions are kept for ```vecbackup deleteoldversions```?
* Keeps all versions within one day
* Keep one version per hour for the last week
* Keep one version per day in the last month
* Keep one version per week in the last year
* Keep one version per month otherwise
* All extra versions are deleted
* The unused chunk files are not deleted until you run ```vecbackup purgeunused```.

### Q: Why don't you use XYZ software instead?
* Because various XYZ software have limitations that do not meet my requirements.

### Q: Is this ready for use?
* **This is an alpha release.**
* **The backup file format is still subject to change.**
* **Use at your own risk.**
* Having said that, I have been using it for a few years **in conjunction** with other backup software. I regularly test recovering important data from the backups.

### Q: What do you use this for?
* I use this to backup all my data, mostly consisting of terabytes of irreplaceable photos and videos.
* It is also great for storing sensitive or working data on free cloud storage.
