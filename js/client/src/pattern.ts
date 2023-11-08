
const PATTERN_OP_ALL = 0;
const PATTERN_OP_SOME = 1;
const PATTERN_OP_NONE = 2;
const PATTERN_OP_PUBLISH_ID = 3;
const PATTERN_OP_STREAM_ID = 4;
const PATTERN_OP_TRACK_ID = 5;
const PATTERN_OP_TRACK_RID = 6;
const PATTERN_OP_TRACK_LABEL_ALL_MATCH = 7;
const PATTERN_OP_TRACK_LABEL_SOME_MATCH = 8;
const PATTERN_OP_TRACK_LABEL_NONE_MATCH = 9;
const PATTERN_OP_TRACK_LABEL_ALL_HAS = 10;
const PATTERN_OP_TRACK_LABEL_SOME_HAS = 11;
const PATTERN_OP_TRACK_LABEL_NONE_HAS = 12;
const PATTERN_OP_TRACK_TYPE = 13;

export interface Labels {
    [key: string]: string
}

export interface Pattern {
    op: number;
    args?: string[];
    children?: Pattern[];
}

function All(...pts: Pattern[]): Pattern {
    return {
        op: PATTERN_OP_ALL,
        children: pts,
    };
}

function Some(...pts: Pattern[]): Pattern {
    return {
        op: PATTERN_OP_SOME,
        children: pts,
    };
}

function None(...pts: Pattern[]): Pattern {
    return {
        op: PATTERN_OP_NONE,
        children: pts,
    };
}

function PublishIdIn(...ids: string[]): Pattern {
    return {
        op: PATTERN_OP_PUBLISH_ID,
        args: ids,
    };
}

function StreamIdIn(...ids: string[]): Pattern {
    return {
        op: PATTERN_OP_STREAM_ID,
        args: ids,
    };
}

function GlobalIdIn(...ids: string[]): Pattern {
    return {
        op: PATTERN_OP_TRACK_ID,
        args: ids,
    };
}

function TrackRIdIn(...ids: string[]): Pattern {
    return {
        op: PATTERN_OP_TRACK_RID,
        args: ids,
    };
}

function labelsToArgs(labels: Labels): string[] {
    const args: string[] = [];
    for (const key of Object.keys(labels)) {
        const value = labels[key];
        args.push(key);
        args.push(value);
    }
    return args;
}

function LabelsAllMatch(labels: Labels): Pattern {
    const args = labelsToArgs(labels);
    return {
        op: PATTERN_OP_TRACK_LABEL_ALL_MATCH,
        args,
    };
}

function LabelsSomeMatch(labels: Labels): Pattern {
    const args = labelsToArgs(labels);
    return {
        op: PATTERN_OP_TRACK_LABEL_SOME_MATCH,
        args,
    };
}

function LabelsNoneMatch(labels: Labels): Pattern {
    const args = labelsToArgs(labels);
    return {
        op: PATTERN_OP_TRACK_LABEL_NONE_MATCH,
        args,
    };
}

function LabelsAllExists(...names: string[]): Pattern {
    return {
        op: PATTERN_OP_TRACK_LABEL_ALL_HAS,
        args: names,
    };
}

function LabelsSomeExists(...names: string[]): Pattern {
    return {
        op: PATTERN_OP_TRACK_LABEL_SOME_HAS,
        args: names,
    };
}

function LabelsNoneExists(...names: string[]): Pattern {
    return {
        op: PATTERN_OP_TRACK_LABEL_NONE_HAS,
        args: names,
    };
}

function TrackTypeIn(...types: string[]): Pattern {
    return {
        op: PATTERN_OP_TRACK_TYPE,
        args: types,
    };
}

export default {
    All,
    Some,
    None,
    PublishIdIn,
    StreamIdIn,
    GlobalIdIn,
    TrackRIdIn,
    LabelsAllMatch,
    LabelsSomeMatch,
    LabelsNoneMatch,
    LabelsAllExists,
    LabelsSomeExists,
    LabelsNoneExists,
    TrackTypeIn,
};