#import <React/RCTBridgeModule.h>
#import <React/RCTEventEmitter.h>

@interface RCT_EXTERN_MODULE (LiveActivityModule, RCTEventEmitter)

RCT_EXTERN_METHOD(areEnabled : (RCTPromiseResolveBlock)resolve
                  rejecter : (RCTPromiseRejectBlock)reject)

RCT_EXTERN_METHOD(getPushToken : (RCTPromiseResolveBlock)resolve
                  rejecter : (RCTPromiseRejectBlock)reject)

RCT_EXTERN_METHOD(start : (nonnull NSNumber *)waiting
                  working : (nonnull NSNumber *)working
                  idle : (nonnull NSNumber *)idle
                  title : (nonnull NSString *)title
                  session : (nonnull NSString *)session
                  items : (nonnull NSString *)items
                  server : (nonnull NSString *)server
                  resolver : (RCTPromiseResolveBlock)resolve
                  rejecter : (RCTPromiseRejectBlock)reject)

RCT_EXTERN_METHOD(update : (nonnull NSNumber *)waiting
                  working : (nonnull NSNumber *)working
                  idle : (nonnull NSNumber *)idle
                  title : (nonnull NSString *)title
                  session : (nonnull NSString *)session
                  items : (nonnull NSString *)items)

RCT_EXTERN_METHOD(end)

@end
