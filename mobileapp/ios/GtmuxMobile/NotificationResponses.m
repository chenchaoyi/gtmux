#import <React/RCTBridgeModule.h>

@interface RCT_EXTERN_MODULE(GtmuxNotificationResponses, NSObject)
RCT_EXTERN_METHOD(drain:(RCTPromiseResolveBlock)resolve rejecter:(RCTPromiseRejectBlock)reject)
RCT_EXTERN_METHOD(acknowledge:(NSString *)identifier action:(NSString *)action)
RCT_EXTERN_METHOD(pendingSceneLinks:(RCTPromiseResolveBlock)resolve rejecter:(RCTPromiseRejectBlock)reject)
RCT_EXTERN_METHOD(acknowledgeSceneLink:(NSString *)identifier)
@end
