
import volpe_container_pb2 as pb
import common_pb2 as pbc
import volpe_container_pb2_grpc as vp
import grpc
import concurrent.futures

class VolpeGreeterServicer(vp.VolpeContainerServicer):
    def SayHello(self, request: pb.HelloRequest, context):
        return pb.HelloReply(message="hello " + request.name)
    def InitFromSeed(self, request: pb.Seed, context):
        """Missing associated documentation comment in .proto file."""
        return pb.Reply(success=True, message="dummy response")

    def InitFromSeedPopulation(self, request, context):
        """Missing associated documentation comment in .proto file."""
        return pb.Reply(success=True, message="dummy response")

    def GetBestPopulation(self, request, context):
        """Missing associated documentation comment in .proto file."""
        return pbc.Population(members=[], problemID="p1")

    def AdjustPopulationSize(self, request, context):
        """Missing associated documentation comment in .proto file."""
        return pb.Reply(success=True, message="dummy response")

    def RunForGenerations(self, request, context):
        """Missing associated documentation comment in .proto file."""
        return pb.Reply(success=True, message="dummy response")
    
server = grpc.server(concurrent.futures.ThreadPoolExecutor(max_workers=10))
vp.add_VolpeContainerServicer_to_server(VolpeGreeterServicer(), server)
server.add_insecure_port("0.0.0.0:8081")
server.start()
server.wait_for_termination()
