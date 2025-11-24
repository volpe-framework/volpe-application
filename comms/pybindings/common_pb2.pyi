from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Individual(_message.Message):
    __slots__ = ("genotype", "fitness")
    GENOTYPE_FIELD_NUMBER: _ClassVar[int]
    FITNESS_FIELD_NUMBER: _ClassVar[int]
    genotype: bytes
    fitness: float
    def __init__(self, genotype: _Optional[bytes] = ..., fitness: _Optional[float] = ...) -> None: ...

class Population(_message.Message):
    __slots__ = ("members", "problemID")
    MEMBERS_FIELD_NUMBER: _ClassVar[int]
    PROBLEMID_FIELD_NUMBER: _ClassVar[int]
    members: _containers.RepeatedCompositeFieldContainer[Individual]
    problemID: str
    def __init__(self, members: _Optional[_Iterable[_Union[Individual, _Mapping]]] = ..., problemID: _Optional[str] = ...) -> None: ...

class ImageStreamObject(_message.Message):
    __slots__ = ("details", "chunk")
    DETAILS_FIELD_NUMBER: _ClassVar[int]
    CHUNK_FIELD_NUMBER: _ClassVar[int]
    details: ImageDetails
    chunk: ImageChunk
    def __init__(self, details: _Optional[_Union[ImageDetails, _Mapping]] = ..., chunk: _Optional[_Union[ImageChunk, _Mapping]] = ...) -> None: ...

class ImageDetails(_message.Message):
    __slots__ = ("problemID", "imageSizeBytes")
    PROBLEMID_FIELD_NUMBER: _ClassVar[int]
    IMAGESIZEBYTES_FIELD_NUMBER: _ClassVar[int]
    problemID: str
    imageSizeBytes: int
    def __init__(self, problemID: _Optional[str] = ..., imageSizeBytes: _Optional[int] = ...) -> None: ...

class ImageChunk(_message.Message):
    __slots__ = ("data",)
    DATA_FIELD_NUMBER: _ClassVar[int]
    data: bytes
    def __init__(self, data: _Optional[bytes] = ...) -> None: ...
