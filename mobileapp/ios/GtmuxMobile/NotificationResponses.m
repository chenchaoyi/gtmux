#import <React/RCTBridgeModule.h>
#import <React/RCTEventEmitter.h>

@interface RCT_EXTERN_MODULE(GtmuxNotificationResponses, RCTEventEmitter)
RCT_EXTERN_METHOD(drain:(RCTPromiseResolveBlock)resolve rejecter:(RCTPromiseRejectBlock)reject)
RCT_EXTERN_METHOD(acknowledge:(NSString *)identifier action:(NSString *)action)
RCT_EXTERN_METHOD(pendingSceneLinks:(RCTPromiseResolveBlock)resolve rejecter:(RCTPromiseRejectBlock)reject)
RCT_EXTERN_METHOD(acknowledgeSceneLink:(NSString *)identifier)
RCT_EXTERN_METHOD(currentSceneMetrics:(RCTPromiseResolveBlock)resolve rejecter:(RCTPromiseRejectBlock)reject)
@end
