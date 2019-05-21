# DEFUNCT

This project has been superceded by https://github.com/helix-collective/hx-deploy-tool

# camus
Deployments that don't make you want to kill yourself

# conceptual model
- client/server: server manages application deploys, client connects to it to run commands, get info, etc.
- multiple deploys can be run at once, each will be on a different port. server monitors/tracks the deploys
- a particular deploy can be selected as the "live" deploy. camus runs an instance of haproxy to forward requests from the "frontend" port to the "live" deploy. see the ports section below for more information.
- rollbacks, roll-forwards are fast and easy - camus just reconfigures and reloads hxproxy with the backend port pointed to the current deploy.

# basic server setup
- install a recent version of go (1.4.2+ is good) & haproxy (1.5+ is good)
- install camus using go get / go install
- setup an empty deploys directory with a minimal '{}' config.json file
- run camus -server pointed at that directory
- point your frontend webserver (e.g. nginx reverse proxy) to the
  haproxy frontend port (default 8098 - see ports below) once
  an application has been deployed and selected to run.

For convenience, example-setup has two install scripts that automate
one way of doing the install steps, and a Dockerfile that strings it
all together in a docker container as an example (note however that
this does not constitute a recommended docker setup of camus).
Use test.sh to run the docker build.

# basic app setup
- create a deploy.json file, see testapp/deploy.json for an example.


Explanation of the format (note that the comments will make it invalid json)
```
{
  # some name 
  "Name": "MyApp",

  # command to build the app
  "BuildCmd": "./build.sh", 

  # directory containing the built app
  "BuildOutputDir": "./build",

  # command to start the server.
  # it will be run from the base of the built app directory.
  # the %PORT% pattern will be substituted with the port camus
  # wants to run the server on.  The application must honour this,
  # and it must connect quickly
  "RunCmd": "node app.js %PORT%",  # command to start the server

  # Http endpoint to use for health checks
  "HealthEndpoint": "/status",

  # Deploy targets.
  "Targets": {

    # Keys are anything you want.
    # Though "prod" is the default when running the client, so this
    # is the most convenient one.
    "prod": {

      # ssh user and domain string 
      # to use for rsync and to tunnel to the camus daemon
      "Ssh": "user@myapp.com",

      # optional port
      "SshPort": 22,

      # camus base port on the server. 
      # (specified with -port when running the server)
      "Base": 8000  # base 
    }
  }
}
```

# example usage

```camus -h```

Show help


```camus -server -enforce -serverRoot my-deploys```

Start the camus server on the default port range

# port range
The default port range is 100 ports, and starts at 8000.
- The camus daemon itself will run at the base.
- haproxy's status output will run at the top (default 8099)
- haproxy's frontend will run at the top-1 (default 8098)
- applications will start up on the first free port in that range.
  e.g. the first will run on 8001, the second on 8002... if the
  first is killed then 8001 will be available again. So there are 
  initially 97 available ports out of 100, which should be plenty
  (normally only need 2-3 at a time).
- camus will only listen on localhost, everything else on all ips.
  use a firewall/security group to lock down the rest if desired. 
  (but no need for camus).



