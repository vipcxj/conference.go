"use strict";
var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
var __generator = (this && this.__generator) || function (thisArg, body) {
    var _ = { label: 0, sent: function() { if (t[0] & 1) throw t[1]; return t[1]; }, trys: [], ops: [] }, f, y, t, g;
    return g = { next: verb(0), "throw": verb(1), "return": verb(2) }, typeof Symbol === "function" && (g[Symbol.iterator] = function() { return this; }), g;
    function verb(n) { return function (v) { return step([n, v]); }; }
    function step(op) {
        if (f) throw new TypeError("Generator is already executing.");
        while (g && (g = 0, op[0] && (_ = 0)), _) try {
            if (f = 1, y && (t = op[0] & 2 ? y["return"] : op[0] ? y["throw"] || ((t = y["return"]) && t.call(y), 0) : y.next) && !(t = t.call(y, op[1])).done) return t;
            if (y = 0, t) op = [op[0] & 2, t.value];
            switch (op[0]) {
                case 0: case 1: t = op; break;
                case 4: _.label++; return { value: op[1], done: false };
                case 5: _.label++; y = op[1]; op = [0]; continue;
                case 7: op = _.ops.pop(); _.trys.pop(); continue;
                default:
                    if (!(t = _.trys, t = t.length > 0 && t[t.length - 1]) && (op[0] === 6 || op[0] === 2)) { _ = 0; continue; }
                    if (op[0] === 3 && (!t || (op[1] > t[0] && op[1] < t[3]))) { _.label = op[1]; break; }
                    if (op[0] === 6 && _.label < t[1]) { _.label = t[1]; t = op; break; }
                    if (t && _.label < t[2]) { _.label = t[2]; _.ops.push(op); break; }
                    if (t[2]) _.ops.pop();
                    _.trys.pop(); continue;
            }
            op = body.call(thisArg, _);
        } catch (e) { op = [6, e]; y = 0; } finally { f = t = 0; }
        if (op[0] & 5) throw op[1]; return { value: op[0] ? op[1] : void 0, done: true };
    }
};
Object.defineProperty(exports, "__esModule", { value: true });
exports.ConferenceClient = void 0;
require("webrtc-adapter");
var socket_io_client_1 = require("socket.io-client");
function splitUrl(url) {
    var spos = url.indexOf('://');
    var startPos = 0;
    if (spos != -1) {
        startPos = spos + 3;
    }
    var pos = url.indexOf('/', startPos);
    if (pos != -1) {
        return [url.substring(0, pos), url.substring(pos)];
    }
    else {
        return [url, '/'];
    }
}
var ConferenceClient = /** @class */ (function () {
    function ConferenceClient(signalUrl, token, polite) {
        if (polite === void 0) { polite = true; }
        var _this = this;
        this.ark = function (func) {
            if (func) {
                func();
            }
        };
        this.makeSurePeer = function () {
            if (!_this.peer) {
                var peer_1 = new RTCPeerConnection();
                peer_1.onnegotiationneeded = function () { return __awaiter(_this, void 0, void 0, function () {
                    var desc, msg, err_1;
                    return __generator(this, function (_a) {
                        switch (_a.label) {
                            case 0:
                                _a.trys.push([0, 2, 3, 4]);
                                this.makingOffer = true;
                                return [4 /*yield*/, peer_1.setLocalDescription()];
                            case 1:
                                _a.sent();
                                desc = peer_1.localDescription;
                                msg = {
                                    type: desc.type,
                                    sdp: desc.sdp,
                                };
                                this.socket.emit("sdp", msg);
                                return [3 /*break*/, 4];
                            case 2:
                                err_1 = _a.sent();
                                console.error(err_1);
                                return [3 /*break*/, 4];
                            case 3:
                                this.makingOffer = false;
                                return [7 /*endfinally*/];
                            case 4: return [2 /*return*/];
                        }
                    });
                }); };
                peer_1.onicecandidate = function (evt) {
                    var msg;
                    if (evt.candidate) {
                        msg = {
                            op: "add",
                            candidate: evt.candidate.toJSON(),
                        };
                    }
                    else {
                        msg = {
                            op: "end",
                        };
                    }
                    _this.socket.emit("candidate", msg);
                };
                _this.socket.on("sdp", function (msg, ark) { return __awaiter(_this, void 0, void 0, function () {
                    var offerCollision, _i, _a, pending, desc, send_msg;
                    return __generator(this, function (_b) {
                        switch (_b.label) {
                            case 0:
                                this.ark(ark);
                                offerCollision = msg.type === "offer"
                                    && (this.makingOffer || peer_1.signalingState !== "stable");
                                this.ignoreOffer = !this.polite && offerCollision;
                                if (this.ignoreOffer) {
                                    return [2 /*return*/];
                                }
                                return [4 /*yield*/, peer_1.setRemoteDescription({
                                        type: msg.type,
                                        sdp: msg.sdp,
                                    })];
                            case 1:
                                _b.sent();
                                _i = 0, _a = this.pendingCandidates;
                                _b.label = 2;
                            case 2:
                                if (!(_i < _a.length)) return [3 /*break*/, 5];
                                pending = _a[_i];
                                return [4 /*yield*/, this.addCandidate(peer_1, pending)];
                            case 3:
                                _b.sent();
                                _b.label = 4;
                            case 4:
                                _i++;
                                return [3 /*break*/, 2];
                            case 5:
                                if (!(msg.type === 'offer')) return [3 /*break*/, 7];
                                return [4 /*yield*/, peer_1.setLocalDescription()];
                            case 6:
                                _b.sent();
                                desc = peer_1.localDescription;
                                send_msg = {
                                    type: desc.type,
                                    sdp: desc.sdp,
                                };
                                this.socket.emit("sdp", send_msg);
                                _b.label = 7;
                            case 7: return [2 /*return*/];
                        }
                    });
                }); });
                _this.socket.on("candidate", function (msg, ark) { return __awaiter(_this, void 0, void 0, function () {
                    return __generator(this, function (_a) {
                        switch (_a.label) {
                            case 0:
                                this.ark(ark);
                                if (!peer_1.remoteDescription) {
                                    this.pendingCandidates.push(msg);
                                    return [2 /*return*/];
                                }
                                return [4 /*yield*/, this.addCandidate(peer_1, msg)];
                            case 1:
                                _a.sent();
                                return [2 /*return*/];
                        }
                    });
                }); });
                _this.peer = peer_1;
            }
            return _this.peer;
        };
        this.addCandidate = function (peer, msg) { return __awaiter(_this, void 0, void 0, function () {
            var err_2;
            return __generator(this, function (_a) {
                switch (_a.label) {
                    case 0:
                        _a.trys.push([0, 5, , 6]);
                        if (!(msg.op == "end")) return [3 /*break*/, 2];
                        return [4 /*yield*/, peer.addIceCandidate()];
                    case 1:
                        _a.sent();
                        return [3 /*break*/, 4];
                    case 2: return [4 /*yield*/, peer.addIceCandidate(msg.candidate)];
                    case 3:
                        _a.sent();
                        _a.label = 4;
                    case 4: return [3 /*break*/, 6];
                    case 5:
                        err_2 = _a.sent();
                        if (!this.ignoreOffer) {
                            throw err_2;
                        }
                        return [3 /*break*/, 6];
                    case 6: return [2 /*return*/];
                }
            });
        }); };
        this.publish = function (stream) { return __awaiter(_this, void 0, void 0, function () {
            var peer;
            return __generator(this, function (_a) {
                this.socket.connect();
                peer = this.makeSurePeer();
                stream.getTracks().forEach(function (track) {
                    peer.addTrack(track, stream);
                });
                return [2 /*return*/];
            });
        }); };
        this.wait = function (evt, _a) {
            var _b = _a === void 0 ? {} : _a, arkData = _b.arkData, timeout = _b.timeout;
            return new Promise(function (resolve) {
                _this.socket.once(evt, function () {
                    var args = [];
                    for (var _i = 0; _i < arguments.length; _i++) {
                        args[_i] = arguments[_i];
                    }
                    var ark = function () { return undefined; };
                    if (args.length > 0 && typeof (args[args.length - 1]) == 'function') {
                        ark = args[args.length - 1];
                        args = args.slice(0, args.length - 1);
                    }
                    if (arkData) {
                        ark(arkData);
                    }
                    else {
                        ark();
                    }
                    if (args.length > 1) {
                        console.warn("Too many response data: ".concat(args.join(", ")));
                    }
                    if (args.length == 0) {
                        resolve(undefined);
                    }
                    else {
                        var res = void 0;
                        try {
                            res = JSON.parse(args[0]);
                        }
                        catch (e) {
                            res = args[0];
                        }
                        resolve(res);
                    }
                });
            });
        };
        var _a = splitUrl(signalUrl), host = _a[0], path = _a[1];
        this.makingOffer = false;
        this.ignoreOffer = false;
        this.polite = polite;
        this.pendingCandidates = [];
        this.streams = [];
        this.socket = (0, socket_io_client_1.io)(host, {
            auth: {
                token: token,
            },
            path: path,
            autoConnect: true,
        });
        this.socket.on('error', function (msg, ark) {
            console.error("Received".concat(msg.fatal ? " fatal " : " ", "error ").concat(msg.msg, " because of ").concat(msg.cause));
            if (ark) {
                ark();
            }
        });
        this.socket.on('stream', function (msg, ark) {
            if (msg.op == "add") {
                console.log("Add stream with id ".concat(msg.stream.id, " and stream id ").concat(msg.stream.streamId));
                _this.streams.push(msg.stream);
            }
            else {
                console.log("Remove stream with id ".concat(msg.stream.id, " and stream id ").concat(msg.stream.streamId));
                _this.streams = _this.streams.filter(function (st) { return st.id == msg.stream.id && st.streamId == msg.stream.streamId; });
            }
        });
        this.socket.onAny(function (evt) {
            var args = [];
            for (var _i = 1; _i < arguments.length; _i++) {
                args[_i - 1] = arguments[_i];
            }
            if (['error', 'stream', 'sdp', 'candidate'].indexOf(evt) !== -1) {
                return;
            }
            var ark = args[args.length - 1];
            var hasArk = typeof ark == 'function';
            if (hasArk) {
                args = args.slice(0, args.length - 1);
            }
            else {
                ark = function () { return undefined; };
            }
            console.log("Received event ".concat(evt, " with args ").concat(args.join(",")));
            ark();
        });
    }
    return ConferenceClient;
}());
exports.ConferenceClient = ConferenceClient;
