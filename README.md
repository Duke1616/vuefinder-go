# vuefinder-go

## Example Launch

### Backend

#### Password Authentication

```
go run main.go --host 127.0.0.1:22 --user user --password "123456"
```

#### Key Authentication

```
go run main.go --host 127.0.0.1:22 --user user --key ~/.ssh/id_rsa
```

#### Using Both Password and Key (Recommended)

If the server supports multiple authentication methods, you can provide both password and key, and the program will try all methods:

```
go run main.go --host 127.0.0.1:22 --user user --password "123456" --key ~/.ssh/id_rsa
```

**Note**: 
- If the password contains special characters (such as `!`, `$`, `&`, etc.), please wrap the password in quotes
- If you encounter authentication failure errors, try using key authentication
- If the server only supports key authentication, you must use the `--key` parameter

### Frontend

```
cd example
pnpm install
pnpm run dev
```

### Access Address

```
http://localhost:5173/
```
