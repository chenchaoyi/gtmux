#import <React/RCTBridgeModule.h>

@interface RCT_EXTERN_MODULE (LiveActivityModule, NSObject)

RCT_EXTERN_METHOD(areEnabled : (RCTPromiseResolveBlock)resolve
                  rejecter : (RCTPromiseRejectBlock)reject)

RCT_EXTERN_METHOD(start : (nonnull NSNumber *)waiting
                  working : (nonnull NSNumber *)working
                  idle : (nonnull NSNumber *)idle
                  title : (nonnull NSString *)title
                  resolver : (RCTPromiseResolveBlock)resolve
                  rejecter : (RCTPromiseRejectBlock)reject)

RCT_EXTERN_METHOD(update : (nonnull NSNumber *)waiting
                  working : (nonnull NSNumber *)working
                  idle : (nonnull NSNumber *)idle
                  title : (nonnull NSString *)title)

RCT_EXTERN_METHOD(end)

@end
