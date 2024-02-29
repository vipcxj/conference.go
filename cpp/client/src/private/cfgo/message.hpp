#ifndef _CFGO_MESSAGE_HPP_
#define _CFGO_MESSAGE_HPP_

#include "cfgo/pattern.hpp"
#include <vector>
#include <map>
#include <string>
#include "sio_message.h"

namespace cfgo {

    struct Track
    {
        std::string type;
        std::string pubId;
        std::string globalId;
        std::string bindId;
        std::string rid;
        std::string streamId;
        std::map<std::string, std::string> labels;

        Track(sio::message::ptr msg);
    };
    

    enum SubOp {
        ADD = 0,
        UPDATE = 1,
        REMOVE = 2
    };

    struct SubscribeAddMessage
    {
        SubOp op;
        std::vector<std::string> reqTypes;
        Pattern pattern;
    };
    
    struct SubscribeRemoveMessage
    {
        SubOp op;
        std::string id;
    };
    
    struct SubscribeResultMessage
    {
        std::string id;
    };
    
    struct SubscribedMessage
    {
        std::string subId;
        std::string pubId;
        int sdpId;
        std::vector<Track> tracks;
    };
    

}

#endif