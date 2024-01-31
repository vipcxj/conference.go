package signal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
	"github.com/vipcxj/conference.go/config"
	"github.com/vipcxj/conference.go/errors"
	"github.com/vipcxj/conference.go/pkg/common"
	"github.com/vipcxj/conference.go/utils"
)

func checkRedisMode(mode string) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "auto" || mode == "standalone" || mode == "cluster" || mode == "sentinel" || mode == "ring" {
		return mode, nil
	} else {
		return mode, errors.InvalidConfig("invalid redis mode %s, support one is auto, standalone, cluster, sentinel or ring", mode)
	}
}

func parseRedisOpts(config config.RedisConfigure) (map[string]*redis.Options, *redis.ClusterOptions, error) {
	if config.Addrs == "" {
		return nil, nil, errors.InvalidConfig("addrs is required for redis config")
	}
	var addrsMap map[string]*redis.Options
	var clusterOpts *redis.ClusterOptions
	clusterOpts, err := redis.ParseClusterURL(config.Addrs)
	if err == nil {
		return addrsMap, clusterOpts, nil
	}
	addrs := strings.Split(config.Addrs, ",")
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		opts, err := redis.ParseURL(addr)
		if err == nil {
			addrsMap[opts.Addr] = opts
		} else {
			addrsMap[addr] = &redis.Options{
				Addr: addr,
			}
		}
	}
	if len(addrsMap) == 0 {
		return nil, nil, errors.InvalidConfig("addrs is required for redis config")
	}
	return addrsMap, clusterOpts, nil
}

type redisAuth struct {
	User string
	Pass string
}

func parseRedisAuth(config config.RedisConfigure) ([]redisAuth, error) {
	users := strings.Split(config.Users, " ")
	users = utils.MapSlice(users, func(user string) (string, bool) {
		user = strings.TrimSpace(user)
		if user == "" {
			return user, true
		} else {
			return user, false
		}
	})
	passes := strings.Split(config.Passes, " ")
	passes = utils.MapSlice(passes, func(pass string) (string, bool) {
		pass = strings.TrimSpace(pass)
		if pass == "" {
			return pass, true
		} else {
			return pass, false
		}
	})
	if len(users) == 0 && len(passes) == 0 {
		return nil, nil
	} else if len(users) == 0 {
		return utils.MapSlice(users, func(user string) (redisAuth, bool) {
			return redisAuth{
				User: user,
			}, false
		}), nil
	} else if len(passes) == 0 {
		return utils.MapSlice(passes, func(pass string) (redisAuth, bool) {
			return redisAuth{
				Pass: pass,
			}, false
		}), nil
	}
	if len(users) == 1 {
		return utils.MapSlice(passes, func(pass string) (mapped redisAuth, remove bool) {
			return redisAuth{
				User: users[0],
				Pass: pass,
			}, false
		}), nil
	} else if len(passes) == 1 {
		return utils.MapSlice(users, func(user string) (mapped redisAuth, remove bool) {
			return redisAuth{
				User: user,
				Pass: passes[0],
			}, false
		}), nil
	} else if len(users) == len(passes) {
		var res []redisAuth
		for i := 0; i < len(users); i++ {
			res = append(res, redisAuth{
				User: users[i],
				Pass: passes[i],
			})
		}
		return res, nil
	} else {
		return nil, errors.InvalidConfig("invalid redis config, the number of users not match passes")
	}
}

func MakeRedisClient(redisConfig config.RedisConfigure) (redis.UniversalClient, error) {

	mode, err := checkRedisMode(redisConfig.Mode)
	if err != nil {
		return nil, err
	}
	addrsMap, clusterOpts, err := parseRedisOpts(redisConfig)
	if err != nil {
		return nil, err
	}

	if mode == "auto" {
		if redisConfig.MasterName != "" {
			mode = "sentinel"
		} else if (clusterOpts != nil && len(clusterOpts.Addrs) > 0) || len(addrsMap) > 1 {
			mode = "cluster"
		} else {
			mode = "standalone"
		}
	}
	var addrs []string
	if clusterOpts != nil {
		addrs = clusterOpts.Addrs
	} else {
		for addr, _ := range addrsMap {
			addrs = append(addrs, addr)
		}
	}
	clientName := config.Conf().ClusterNodeName()
	auths, err := parseRedisAuth(redisConfig)
	if err != nil {
		return nil, err
	}
	var client redis.UniversalClient
	switch mode {
	case "standalone":
		var naddr int = len(addrs)
		if naddr != 1 {
			return nil, errors.InvalidConfig("redis config in standalone mode must specify one and only one address, but got %d addresses: %s", naddr, strings.Join(addrs, ","))
		}
		if len(auths) > 1 {
			return nil, errors.InvalidConfig("invalid redis config, too many users or passes")
		}
		var user, pass string
		if len(auths) == 1 {
			user = auths[0].User
			pass = auths[0].Pass
		}
		var opts *redis.Options
		if clusterOpts != nil {
			addr := strings.TrimSpace(redisConfig.Addrs)
			opts, err = redis.ParseURL(addr)
			if err != nil {
				opts = &redis.Options{
					Addr: addr,
				}
			}
		} else {
			opts = addrsMap[addrs[0]]
		}
		opts.ClientName = clientName
		if user != "" {
			opts.Username = user
		}
		if pass != "" {
			opts.Password = pass
		}
		client = redis.NewClient(opts)
	case "cluster":
		var opts *redis.ClusterOptions
		if clusterOpts != nil {
			if len(auths) > 1 {
				return nil, errors.InvalidConfig("invalid redis config, cluster redis connect string only one auth data, but got %d", len(auths))
			}
			opts = clusterOpts
		} else {
			opts = &redis.ClusterOptions{
				Addrs: addrs,
			}
			if len(auths) > 1 {
				if len(auths) != len(addrs) {
					return nil, errors.InvalidConfig("invalid redis config, the number of auth data not match addrs")
				}
				opts.NewClient = func(opt *redis.Options) *redis.Client {
					addr := opt.Addr
					i := utils.IndexOf(addrs, addr, nil)
					if i == -1 {
						panic(errors.ThisIsImpossible().GenCallStacks(0))
					}
					auth := auths[i]
					if auth.User != "" {
						opts.Username = auth.User
					}
					if auth.Pass != "" {
						opts.Password = auth.Pass
					}
					return redis.NewClient(opt)
				}
			}
		}
		opts.ClientName = clientName
		if len(auths) == 1 {
			auth := auths[0]
			if auth.User != "" {
				opts.Username = auth.User
			}
			if auth.Pass != "" {
				opts.Password = auth.Pass
			}
		}
		client = redis.NewClusterClient(opts)
	case "sentinel":
		if redisConfig.MasterName == "" {
			return nil, errors.InvalidConfig("invalid redis config, masterName is required for sentinel mode")
		}
		if len(auths) > 1 {
			return nil, errors.InvalidConfig("invalid redis config, too many users or passes for sentinel mode")
		}
		var user, pass string
		if len(auths) == 1 {
			user = auths[0].User
			pass = auths[0].Pass
		}
		var opts *redis.FailoverOptions
		if clusterOpts != nil {
			var failoverOpts redis.FailoverOptions
			err = common.ShallowCopy(clusterOpts, &failoverOpts)
			if err != nil {
				return nil, errors.FatalError(err.Error()).GenCallStacks(0)
			}
			opts = &failoverOpts
		} else {
			opts = &redis.FailoverOptions{
				SentinelAddrs: addrs,
			}
		}
		opts.MasterName = redisConfig.MasterName
		opts.ClientName = clientName
		if user != "" {
			opts.Username = user
		}
		if pass != "" {
			opts.Password = pass
		}
		client = redis.NewFailoverClient(opts)
	case "ring":
		var opts *redis.RingOptions
		if clusterOpts != nil {
			var ringOpts redis.RingOptions
			if len(auths) > 1 {
				return nil, errors.InvalidConfig("invalid redis config, ring mode only support one auth data, but got %d", len(auths))
			}
			common.ShallowCopy(clusterOpts, &ringOpts)
			opts = &ringOpts
		} else {
			opts = &redis.RingOptions{
				Addrs: utils.SliceToMap[string, string, string](addrs, func(s string, index int) (key string, value string, remove bool) {
					return strconv.Itoa(index), s, false
				}, nil),
			}
			if len(auths) > 1 {
				if len(auths) != len(addrs) {
					return nil, errors.InvalidConfig("invalid redis config, the number of auth data not match addrs")
				}
				opts.NewClient = func(opt *redis.Options) *redis.Client {
					addr := opt.Addr
					i := utils.IndexOf(addrs, addr, nil)
					if i == -1 {
						panic(errors.ThisIsImpossible().GenCallStacks(0))
					}
					auth := auths[i]
					if auth.User != "" {
						opts.Username = auth.User
					}
					if auth.Pass != "" {
						opts.Password = auth.Pass
					}
					return redis.NewClient(opt)
				}
			}
		}
		opts.ClientName = clientName
		if len(auths) == 1 {
			auth := auths[0]
			if auth.User != "" {
				opts.Username = auth.User
			}
			if auth.Pass != "" {
				opts.Password = auth.Pass
			}
		}
		client = redis.NewRing(opts)
	default:
		return nil, errors.ThisIsImpossible().GenCallStacks(0)
	}
	return client, nil
}

func MakeRedisKey(key string) string {
	prefix := config.Conf().Cluster.Redis.KeyPrefix
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return key
	} else {
		return fmt.Sprintf("%s%s", prefix, key)
	}
}