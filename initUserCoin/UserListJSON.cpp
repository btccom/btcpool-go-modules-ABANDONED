#include <map>
#include <string>
#include <mutex>

#include "UserListJSON.h"

using std::map;
using std::string;
using std::mutex;
using std::lock_guard;

extern "C" {

mutex userIDMapLock;
map<int, string> userIDMap;
string userMapJSON;


void addUser(int puid, const char* puname) {
    lock_guard<mutex> scopeLock(userIDMapLock);
	userIDMap[puid] = puname;
}

const char* getUserListJson(int lastUserId) {
    lock_guard<mutex> scopeLock(userIDMapLock);
    
	map<int, string>::iterator iter;
	if (lastUserId == 0) {
		iter = userIDMap.begin();
	} else {
		iter = userIDMap.upper_bound(lastUserId);
	}
	userMapJSON = "{\"err_no\":0,\"err_msg\":null,\"data\":{";

	while (iter != userIDMap.end()) {
		userMapJSON += '"';
		userMapJSON += iter->second;
		userMapJSON += "\":";
		userMapJSON += std::to_string(iter->first);
		if (++iter != userIDMap.end()) {
			userMapJSON += ',';
		}
	}

	userMapJSON += "}}";

	return userMapJSON.c_str();
}

} // end of extern "C"
