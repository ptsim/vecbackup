# vecbackup

Versioned Encrypted Compressed backup.

## How to use

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

## How to build

* Install golang.
* Clone this repository.
* ```go build```
* ```go test``` (or ```go test -short```)

You will find the ```vecbackup``` binary in the current directory.

