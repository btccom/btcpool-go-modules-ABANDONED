#pragma once

#ifdef __cplusplus
extern "C" {
#endif /* __cplusplus */
 
void addUser(int puid, const char* puname);
const char* getUserListJson(int lastUserId);
 
#ifdef __cplusplus
}
#endif /* __cplusplus */
