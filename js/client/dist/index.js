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
    function ConferenceClient(signalUrl, token) {
        var _this = this;
        this.connect = function () { return __awaiter(_this, void 0, void 0, function () {
            var stream, self, candidateListener, offer, waitForAnswer, sdp;
            var _this = this;
            return __generator(this, function (_a) {
                switch (_a.label) {
                    case 0:
                        this.socket.connect();
                        this.peer = new RTCPeerConnection();
                        return [4 /*yield*/, navigator.mediaDevices.getUserMedia({
                                video: true,
                            })];
                    case 1:
                        stream = _a.sent();
                        stream.getTracks().forEach(function (track) {
                            _this.peer.addTrack(track, stream);
                        });
                        this.peer.onicecandidate = function (evt) {
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
                        self = this;
                        candidateListener = function (candidate, ark) {
                            if (candidate.op == "end") {
                                self.peer.addIceCandidate(undefined);
                                self.socket.off(this);
                            }
                            else {
                                self.peer.addIceCandidate(candidate.candidate);
                            }
                        };
                        this.socket.on("candidate", candidateListener);
                        return [4 /*yield*/, this.peer.createOffer()];
                    case 2:
                        offer = _a.sent();
                        return [4 /*yield*/, this.peer.setLocalDescription(offer)];
                    case 3:
                        _a.sent();
                        waitForAnswer = this.wait("answer");
                        this.socket.emit("offer", {
                            sdp: this.peer.localDescription.sdp
                        });
                        return [4 /*yield*/, waitForAnswer];
                    case 4:
                        sdp = (_a.sent()).sdp;
                        return [4 /*yield*/, this.peer.setRemoteDescription({ type: "answer", sdp: sdp })];
                    case 5:
                        _a.sent();
                        console.log("connected.");
                        return [2 /*return*/];
                }
            });
        }); };
        this.wait = function (evt, _a) {
            var _b = _a === void 0 ? {} : _a, arkData = _b.arkData, timeout = _b.timeout;
            return new Promise(function (resolve, reject) {
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
        this.socket = (0, socket_io_client_1.io)(host, {
            auth: {
                token: token,
            },
            path: path,
        });
    }
    return ConferenceClient;
}());
exports.ConferenceClient = ConferenceClient;
