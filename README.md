Working on a Serverless peet-to-peer protocol network which will have fun point-based interactions

## How to run a quick demo (first commit)
### Terminal 1 (no bootstrap; acts like an initial "seed"):
```
go run ./cmd/park-node -name Ian
# note the Addr it prints, e.g. [::]:61907 -> 127.0.0.1:61907 or 0.0.0.0:61907
```
### Terminal 2
```
go run ./cmd/park-node -name Vee -bootstrap 127.0.0.1:61907
```
### Then, in either terminal, type:
```
/say let's ride a bike blindfolded (jk)
```