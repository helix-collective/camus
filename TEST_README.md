# run tests
go test

# install things
- haproxy

# enable password-less localhost ssh
on OS X go to Settings > Sharing > enable "Remote Login"

set up a key if you don't have one already:

```
ssh-keygen -t rsa #if you don't have a key already

cat ~/.ssh/id_rsa.pub >> ~/.ssh/authorized_keys
chmod og-wx ~/.ssh/authorized_keys
```

