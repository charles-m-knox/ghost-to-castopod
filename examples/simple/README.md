# Simple example

This example connects to both a Ghost mysql and Castopod mariadb database, and performs the necessary queries to update your database to synchronize members.

## Usage

Start by cloning the repository:

```bash
git clone https://github.com/charles-m-knox/ghost-to-castopod.git
cd examples/simple
cp config.example.json config.json
```

Next, you have to open your Ghost database manually and find the `plan_id` values that will be used for your configuration:

```sql
SELECT * FROM members_stripe_customers_subscriptions;
```

Review the results, paying particular attention to the field `plan_id` and construct your `config.json` accordingly. The following example shows a very simple configuration with only 1 podcast and 1 membership tier. The Castopod user with id `1` is an existing account, you can set the `createdBy` and `updatedBy` values to any existing Castopod user's ID (can be found in the database).

```json
{
    "castopodConfig": {
        "sqlConnectionString": "castopod-db-username:password@tcp(127.0.0.1:3307)/castopod-db-name",
        "createdBy": 1,
        "updatedBy": 1
    },
    "plans": {
        "price_Z1K324f2dSyHeaXD5G2G1x29": [1]
    },
    "sqlConnectionString": "ghost-db-username:password@tcp(127.0.0.1:3306)/ghost-db-name",
    "blessedAccounts": {
        "admin@example.com": [1]
    }
}
```

Proceed to build this application and run it:

```bash
go get -v
go build -v
# perform a dry run with the -test and -o flags:
./simple -f config.json -test -o out.txt

# done testing - review out.txt and check the query for yourself!
```

When you're ready to run the real thing, you can remove the `-test` (and you'll probably want to remove the `-o out.txt` field too).

## Tips for connecting to a remote mysql db

If your mysql database is only accessible behind an ssh tunnel, you can use ssh forwarding to open up both the Ghost and Castopod connections, assuming one is on 3306 and the other is on 3307:

```bash
ssh -L 3306:127.0.0.1:3306 -L 3307:127.0.0.1:3307 user@server
```

## Alternative: podman/docker container image

If you prefer not to build from source, you can use the pre-built container image.

You must first create a `config.json` just like above, and ensure that the output directory is going to exist within the container.

```bash
podman run --rm -it \
    -v "$(pwd)/config.json:/config.json:ro" \
    ghcr.io/charles-m-knox/ghost-to-castopod:simple-mysql
```

Note: If you're using an SSH port forwarding mechanism for the mysql database connection, you may want to consider adding `--network host` to the above `podman run` command.

### Building the container image

If you're building from an Arch Linux host, you can use your host system's pacman mirrorlist for faster builds. If not, remove the `-v` flag from the `podman build` command below. It is recommended to run `export GOSUMDB=off`.

```bash
podman build \
    -v "/etc/pacman.d/mirrorlist:/etc/pacman.d/mirrorlist:ro" \
    --build-arg GOSUMDB="${GOSUMDB}" \
    --build-arg GOPROXY="${GOPROXY}" \
    -f containerfile \
    -t ghcr.io/charles-m-knox/ghost-to-castopod:simple-mysql .
```
