
#include "volpe_container.grpc.pb.h"
#include "volpe_container.pb.h"
#include <grpcpp/server_context.h>
#include <grpcpp/support/status.h>

#include <sevobench

#include <mutex>
#include <vector>
#include <pair>
#include <string>

typedef std::pair<std::vector<float>, float> PblmInd;

class VolpeProblem {
public:
    // init
    VolpeProblem() {

    }
    // crossover
    // mutate
    //
};

class VolpeContainerImpl final : public VolpeContainer::Service {
    std::vector<>
public:
    grpc::Status SayHello(grpc::ServerContext *context, HelloRequest *req, HelloReply *reply) {
        std::string replyMsg = "hello, " + req->name() + "!";
        reply->set_message(replyMsg);
        return grpc::Status::OK;
    }

    grpc::Status InitFromSeed(grpc::ServerContext *context, Seed *seed, Reply *reply) {
    }
};
