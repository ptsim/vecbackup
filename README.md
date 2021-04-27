# vecbackup

Versioned Encrypted Compressed backup.

* Backup multiple versions locally or to the cloud or remote destinations using rclone.
* De-duplicates based on content checksums (sha512_256)
* Optionally compresses (zlib)
* Optionally password protect and encrypt backups with authenticated encryption (PBKDF2+NaCl)
* MIT license.
* Supported platforms: MacOS, Linux, Windows 10.

**Disclaimer: Use at your own risk.**

## How to use?

Suppose you want to back up ```/a/mystuff``` to ```/b/mybackup```.

First, initialize the backup repository:

```vecbackup init -r /b/mybackup```

Or, instead, initialize the backup repository with compression mode "slow", chunk size 1048576 and password in the password file ```/d/pwfile```:

```vecbackup init -r /b/mybackup -compress slow -chunk-size 1048576 -pw /d/pwfile```

Or, initialize a remote repository using rclone (```remote:path/to/dir```):

```vecbackup init -r rclone:remote:path/to/dir```

If the repository has been initialized with a password, all other commands must be used with the ```-pw <password file>``` flag.

Do the backup:

```vecbackup backup -r /b/mybackup /a/mystuff /c/myotherstuff```

To see the versions of previous backups:

```vecbackup versions -r /b/mybackup```

To list the files in the backup:

```vecbackup ls -r /b/mybackup```

To restore the latest backup to ```/a/temp```:

```vecbackup restore -r /b/mybackup /a/temp```

To test the restore of the latest backup without writing the recovered files:

```vecbackup restore -r /b/mybackup -verify-only /whatever```

To restore a file or dir ```/a/mystuff/path``` from the backup to ```/a/temp```:

```vecbackup restore -r /b/mybackup -target /a/temp /a/mystuff/path```

To restore an older version of the same file or dir:

```vecbackup restore -version <version> -r /b/mybackup -target /a/temp /a/mystuff/path```

To verify that the all backup files of all versions are not corrupted:

```vecbackup verify-repo -r /b/mybackup```

To quickly verify that the all backup files of all versions are not missing chunks:

```vecbackup verify-repo -r /b/mybackup -quick```

To delete old backup versions and reuse the space:

```vecbackup delete-old-versions -r /b/mybackup```

```vecbackup purge-unused -r /b/mybackup```

## How to install?

Download the latest OS X, Linux and Windows releases here:
https://github.com/ptsim/vecbackup/releases

## How to build?

* Install golang.
* ```git clone https://github.com/ptsim/vecbackup.git```
* ```cd vecbackup```
* ```go test ./...```
* ```go build ./cmd/vecbackup```

You will find the ```vecbackup``` binary in the current directory.

## FAQ

### Q: How do I see all the options?
* Run ```vecbackup``` to print all the commands and options.
* Run ```vecbackup help``` for more detailed description of all the commands.

### Q: How are files backed up?
* Each file is broken into 16MB chunks. The size can be set with -chunk-size flag during initialization.
* Each file is recorded as a list of chunks, metadata and whole file checksum.
* Each chunk is checksummed (sha512_256), optionally compressed (zlib) and then optionally encrypted using Golang secretbox (NaCl).
* Chunks are added and never modified or deleted during the backup operation
* De-duplication is based on the content checksum of the chunks before compression and encryption.
* A version manifest file (modified RFC3339Nano timestamp) lists all the files for a version of the backup.

### Q: How does vecbackup know if files have been modified?
* vecbackup assumes that a file has not been modified if its file size and modified timestamp have not changed from the last backup.
* Use the ```backup -force``` to force a backup of every file even the file was already in the repository. This is slow.

### Q: What about symbolic links, hard links, special files, empty directories and other special stuff?
* Symbolic links are backed up. It records the target location of the link.
* Hard links are backed up like normal files.
* Empty directories are backed up.
* Other special files are ignored silently.
* Unix permissions are recorded and recreated except that the directories will be user writable.
* User and group ownership are ignored.
* Last modified timestamp for files are backed up.

### Q: How are files compressed?
* There are four modes of compression:yes, no, slow, auto. The default mode is auto.
   * auto : Compresses most chunks but skip small chunks
            and only check if compression saves space on
            a small prefix of large chunks.
   * slow : Tries to every chunk. Keeps the uncompressed
            version if it is smaller.
   * no   : Never compress chunks.
   * yes  : Compress all chunks.

### Q: How do I check if my repository is valid and not corrupted or missing files?
* Do ```vecbackup restore -r <repository> -verify-only```. This does the equivalent of restoring the latest version in the repository except actually writing the files. All files will be reconstructed by from the compressed and encrypted chunks and verified against the stored checksums.
* Do ```vecbackup verify-repo -r <repository>```. This checks the checksums of every chunk in every version in the repository and produces a summary of chunks in each version. Optionally, use the ```-quick``` flag to check the existence of the chunks only without verifying the chunk checksums.

### Q: Can I still restore if some chunks in the repository are missing or corrupted?
* ```vecbackup restore``` tries to restore as many files as possible even if some chunks are missing or corrupted.
* Any file that cannot be reconstructed will be reported.
* All other files/symbolic links/dirs will still be restored.

### How do I automate or schedule my backups?
* I used crontab to run the backups automatically.
* When a backup is running, it maintains a ```lock``` file in the repository to prevent another instance from backing up to the same repository. This makes it easy to run timed backups without worrying about previous backups taking too long to complete.
* If a backup crashes for some reason, you may have to remove the ```lock``` file manually using the ```vecbackup remove-lock``` command.
* The lock file is only used for the ```vecbackup backup``` command. The other commands can be run concurrently.

### Q: Can I have multiple "backup sets"?
* Yes, just backup different data to different backup repositories.

### Q: How do I know if the files are recovered correctly?
* Each chunk has a sha512_256 checksum.
* Each file has a sha512_256 for the whole file.
* During recovery, the checksums are verified.
* If encryption is used, all chunks, symbolic links, metadata and version manifest files are encrypted using authenticated encryption (See NaCl).
* If no encryption is used, symbolic links, metadata and version manifest files are not checksummed.

### Q: How do I use encryption?
* Create a file containing your desired password
* Use ```-pw <password_file>``` for all commands. For example:

```vecbackup init -pw /a/mybkpw -r /b/mybackup```
* Use the ```-pbkdf2-iterations <num>``` flag for the init command to set how slow key generation and key verification is. The larger the number, the slower it is. Default and minimum 100,000.
* If you lose your password, there is almost no way to recover the data in the backup.

### Q: What is the encryption for?
* So that I can copy the backups to "unsafe" remote, cloud or offline storage.
* With authenticated encryption, I can be sure the backup files have not been modified accidentially or intentionally.

### Q: What's the security objective?
* Make it hard to decrypt the data without the password if a bad actor gets hold of a copy of the repository. (Slow key derivation)
* Make it easy to detect if a bad actor has tempered with my copy of the repository. (Authenticated encryption)

### Q: Did you roll your own encryption scheme?
* No.
* The 256-bit master encryption key is derived from the user's password using PBKDF2.
* The master encryption key is used to decrypt the config file.
* The config file contains a 256-bit storage encryption key and a fingerprint secret.
* All other data is compressed and then encrypted using the storage encryption key.
* Encryption is done using Golang's secretbox module.
* Secretbox provides authenticated encryption and is interoperable with NaCl (https://nacl.cr.yp.to/).
* Chunks are named with the sha512_256(fingerprint secret + sha512_256(original chunk content)). 

## Q: How do I tell vecbackup to exclude certain files?
* Use the -exclude-from <exclude_file> option to the backup command.
* Each line in the <exclude_file> is a pattern containing files to ignore.
* Run ```vecbackup help``` for more details.
* Example file:
``` 
.DS_Store
/a/abc/*
*~
```
* Note: On Windows, use ```\``` as the path separator. On Linux and MacOS, use ```/```.

### Q: Just show me the effects of the operations, aka dry run mode?
* ```vecbackup backup -n ...```
* ```vecbackup restore -n ...```
* ```vecbackup delete-old-versions -n ...```
* ```vecbackup purge-unused -n ...```

### Q: Is this multi-threaded?
* By default, the ```backup```, ```restore``` and ```verify-repo``` commands run multiple operations in parallel (multi-threaded).
* The ```-max-dop``` flag sets the maximum degree of parallellism. The default is 3.
* If you increase the degree of parallelism, it increases memory, cpu, disk and network utilization.
* If ```-max-dop``` is 1, the command will be single threaded.
* All other commands are always single threaded.

### Q: Which older versions are kept for ```vecbackup delete-old-versions```?
* Keeps all versions within one day
* Keep one version per hour for the last week
* Keep one version per day in the last month
* Keep one version per week in the last year
* Keep one version per month otherwise
* All extra versions are deleted
* The unused chunk files are not deleted until you run ```vecbackup purge-unused```.

### Q: Are repositories compatible across platforms (Linux/MacOS/Windows)?
* Yes. You can restore files from a repository that was created on a different platform.
* Use the path sperator for the current platform when specifying paths and excluded file patterns.
* Files are backed up and restored verbatim. Files are treated as a bunch of bytes and not interpreted. In particular, no line ending conversion, unicode normalization or any other change is done when backing up or restoring. File names are also recorded as-is.

### Q: Can this back up directly to the cloud?
* Yes!
* Configure a ```rclone``` remote for the cloud storage provider. Check that the remote works with ```rclone```. See ```rclone``` documentation.
* Use ```-r rclone:remote:path/to/dir``` flag for indicate the remote repository.
* Use ```-rclone-binary <path-to-rclone>``` to set the path of the ```rclone``` program.
* Use ```-lock-file <path-to-lock-file>``` flag to the ```backup``` command if you want to use a local lock file.
* Note: the lock file is only used for the ```backup``` command. Using a remote lock file is most likely not safe against race conditions. ```rclone``` commands are probably not atomic. However, running two backups to the same repository at the same time is fine although it is not recommended.
* The layout within the remote path is identical to a local repository.
* You can ```rclone sync``` a remote repository to a local directory and then use it as a local repository and vice versa.
* This has only been tested using the S3 rclone backend with Wasabi's cloud storage.

### Q: Why don't you use <...> backup software instead?
* Because various backup software have limitations that do not meet my requirements.
* This was a pet project started many years ago when options were more limited and it has gone through many rewrites as requirements change.

### Q: Is this ready for use?
* **This is an alpha release.**
* **The backup file format is still subject to change.**
* **Use at your own risk.**
* Having said that, I have been using it for a few years **in conjunction** with other backup software. I regularly test restoring data from the backups.
* There are also comprehensive unit tests.

### Q: What do you use this for?
* I use this to backup all my data, mostly consisting of terabytes of irreplaceable photos and videos.
* I also use this with rclone to store sensitive or working data in cloud storage.

### Q: Maintenance/Future plans?
* I plan to use and maintain this for a long time.
* A few potential new features:
   * Support Zstandard compression
   * Backup to and restore from a ssh/scp repository (without using rclone)
   * Local cacheing of data when using a remote repository.


