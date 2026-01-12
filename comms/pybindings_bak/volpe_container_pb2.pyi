import common_pb2 as _common_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ResultIndividual(_message.Message):
    __slots__ = ("representation", "fitness")
    REPRESENTATION_FIELD_NUMBER: _ClassVar[int]
    FITNESS_FIELD_NUMBER: _ClassVar[int]
    representation: str
    fitness: float
    def __init__(self, representation: _Optional[str] = ..., fitness: _Optional[float] = ...) -> None: ...

class ResultPopulation(_message.Message):
    __slots__ = ("members",)
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    members: _containers.RepeatedCompositeFieldContainer[ResultIndividual]
    def __init__(self, members: _Optional[_Iterable[_Union[ResultIndividual, _Mapping]]] = ...) -> None: ...

class HelloRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class Seed(_message.Message):
    __slots__ = ("seed",)
    SEED_FIELD_NUMBER: _ClassVar[int]
    seed: int
    def __init__(self, seed: _Optional[int] = ...) -> None: ...

class PopulationSize(_message.Message):
    __slots__ = ("size",)
    SIZE_FIELD_NUMBER: _ClassVar[int]
    size: int
    def __init__(self, size: _Optional[int] = ...) -> None: ...

class HelloReply(_message.Message):
    __slots__ = ("message",)
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    message: str
    def __init__(self, message: _Optional[str] = ...) -> None: ...

class Reply(_message.Message):
    __slots__ = ("success", "message")
    SUCCESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    success: bool
    message: str
    def __init__(self, success: bool = ..., message: _Optional[str] = ...) -> None: ...
