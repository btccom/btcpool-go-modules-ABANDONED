#include <map>
#include <string>
#include <mutex>
#include <time.h>

#include "UserListJSON.h"

using std::lock_guard;
using std::map;
using std::mutex;
using std::string;

extern "C"
{
	struct JSONCache
	{
		int lastUserId;
		string json;
	};

	mutex userIDMapLock;
	map<string /* coin */, map<int /* puid */, string /* puname */>> userIDMaps;
	map<string /* coin */, map<string /* puname */, time_t /* updateTime */>> userUpdateTimeMaps;
	map<string /* coin */,  JSONCache> userListJsonCaches;

	void addUser(int puid, const char *puname, const char *coin)
	{
		lock_guard<mutex> scopeLock(userIDMapLock);
		time_t now = time(nullptr);

		userIDMaps[coin][puid] = puname;
		userIDMaps[""][puid] = puname; // merged list

		userUpdateTimeMaps[coin][puname] = now;
		userUpdateTimeMaps[""][puname] = now; // merged list

		// clear caches
		userListJsonCaches.erase(coin);
		userListJsonCaches.erase(""); // cache for merged list
	}

	const char *getUserListJson(int lastUserId, const char *coin)
	{
		lock_guard<mutex> scopeLock(userIDMapLock);

		auto &cache = userListJsonCaches[coin];
		if (!cache.json.empty() && cache.lastUserId == lastUserId) {
			return cache.json.c_str();
		}
		cache.lastUserId = lastUserId;

		auto &userIDMap = userIDMaps[coin];
		map<int, string>::iterator iter;
		if (lastUserId <= 0)
		{
			iter = userIDMap.begin();
		}
		else
		{
			iter = userIDMap.upper_bound(lastUserId);
		}

		cache.json = "{\"err_no\":0,\"err_msg\":null,\"data\":{";
		while (iter != userIDMap.end())
		{
			cache.json += '"';
			cache.json += iter->second;
			cache.json += "\":";
			cache.json += std::to_string(iter->first);
			if (++iter != userIDMap.end())
			{
				cache.json += ',';
			}
		}
		cache.json += "}}";

		return cache.json.c_str();
	}

	int64_t getUserUpdateTime(const char *puname, const char *coin) {
		auto itr = userUpdateTimeMaps[coin].find(puname);
		if (itr == userUpdateTimeMaps[coin].end()) {
			return 0;
		}
		return itr->second;
	}

} // end of extern "C"
