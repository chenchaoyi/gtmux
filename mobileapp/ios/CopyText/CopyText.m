#import <React/RCTViewManager.h>

@interface RCT_EXTERN_MODULE (CopyTextViewManager, RCTViewManager)
RCT_EXPORT_VIEW_PROPERTY(copyText, NSString)
RCT_EXPORT_VIEW_PROPERTY(copyLabel, NSString)
@end
