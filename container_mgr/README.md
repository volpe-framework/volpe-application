# container_mgr
- **ContainerManager**
  - owns all problem containers on a node
  - tracks active problems, images, and instances
  - handles scaling events and population distribution
- **ProblemContainer**
  - wraps a single Podman container instance
  - manages gRPC communication with the container
  - periodically fetches results or runs generations
  - broadcasts results to registered listeners

<i>Each problem can have N containers. Populations are split evenly across instances<i>
