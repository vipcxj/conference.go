from enum import IntEnum
from dataclasses import dataclass
from typing import Sequence

class PatternOp(IntEnum):
    ALL = 0
    SOME = 1
    NONE = 2
    PUBLISH_ID = 3
    STREAM_ID = 4
    TRACK_ID = 5
    TRACK_RID = 6
    TRACK_LABEL_ALL_MATCH = 7
    TRACK_LABEL_SOME_MATCH = 8
    TRACK_LABEL_NONE_MATCH = 9
    TRACK_LABEL_ALL_HAS = 10
    TRACK_LABEL_SOME_HAS = 11
    TRACK_LABEL_NONE_HAS = 12
    TRACK_TYPE = 13
    
@dataclass
class Pattern:
    op: PatternOp
    args: list[str] | None = None
    children: list["Pattern"] | None = None
    
    @staticmethod
    def All(*pats: "Pattern") -> "Pattern":
        return Pattern(
            op=PatternOp.ALL,
            children=list(pats),
        )
    
    @staticmethod    
    def Some(*pats: "Pattern") -> "Pattern":
        return Pattern(
            op=PatternOp.SOME,
            children=list(pats),
        )
    
    @staticmethod  
    def Nonee(*pats: "Pattern") -> "Pattern":
        return Pattern(
            op=PatternOp.NONE,
            children=list(pats),
        )
        
    @staticmethod
    def PublishIdIn(*ids: str) -> "Pattern":
        return Pattern(
            op=PatternOp.PUBLISH_ID,
            args=list(ids),
        )
        
    @staticmethod
    def StreamIdIn(*ids: str) -> "Pattern":
        return Pattern(
            op = PatternOp.STREAM_ID,
            args = list(ids),
        )

    @staticmethod
    def GlobalIdIn(*ids: str) -> "Pattern":
        return Pattern(
            op = PatternOp.TRACK_ID,
            args = list(ids),
        )

    @staticmethod
    def TrackRIdIn(*ids: str) -> "Pattern":
        return Pattern(
            op = PatternOp.TRACK_RID,
            args = list(ids),
        )

    @staticmethod
    def labelsToArgs(labels: dict[str, str]) -> list[str]:
        args: list[str] = []
        for key in labels:
            value = labels[key]
            args.append(key)
            args.append(value)
        return args

    @staticmethod
    def LabelsAllMatch(labels: dict[str, str]) -> "Pattern":
        args = Pattern.labelsToArgs(labels)
        return Pattern(
            op = PatternOp.TRACK_LABEL_ALL_MATCH,
            args = args,
        )

    @staticmethod
    def LabelsSomeMatch(labels: dict[str, str]) -> "Pattern":
        args = Pattern.labelsToArgs(labels)
        return Pattern(
            op = PatternOp.TRACK_LABEL_SOME_MATCH,
            args = args,
        )

    @staticmethod
    def LabelsNoneMatch(labels: dict[str, str]) -> "Pattern":
        args = Pattern.labelsToArgs(labels)
        return Pattern(
            op = PatternOp.TRACK_LABEL_NONE_MATCH,
            args = args,
        )

    @staticmethod
    def LabelsAllExists(*names: str) -> "Pattern":
        return Pattern(
            op = PatternOp.TRACK_LABEL_ALL_HAS,
            args = list(names),
        )

    @staticmethod
    def LabelsSomeExists(*names: str) -> "Pattern":
        return Pattern(
            op = PatternOp.TRACK_LABEL_SOME_HAS,
            args = list(names),
        )

    @staticmethod
    def LabelsNoneExists(*names: str) -> "Pattern":
        return Pattern(
            op = PatternOp.TRACK_LABEL_NONE_HAS,
            args = list(names),
        )

    @staticmethod
    def TrackTypeIn(*types: str) -> "Pattern":
        return Pattern(
            op = PatternOp.TRACK_TYPE,
            args = list(types),
        )
