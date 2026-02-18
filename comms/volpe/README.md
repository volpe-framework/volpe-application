#comms

## worker → master

workers connect to the master using a bidirectional grpc stream.

when a worker connects, it:
- sends (worker id, cpu count, memory)

after that, it periodically sends:
- `devicemetricsmessage`
- `population` 

## master → worker

the master can send:
- `adjustinstancesmessage` 
- seed populations for distributed evolution
