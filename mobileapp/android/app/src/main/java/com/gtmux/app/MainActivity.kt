package com.gtmux.app

import android.view.KeyEvent
import android.view.ActionMode
import com.facebook.react.ReactApplication
import com.facebook.react.modules.core.DeviceEventManagerModule
import com.facebook.react.ReactActivity
import com.facebook.react.ReactActivityDelegate
import com.facebook.react.defaults.DefaultNewArchitectureEntryPoint.fabricEnabled
import com.facebook.react.defaults.DefaultReactActivityDelegate

class MainActivity : ReactActivity() {
  override fun getMainComponentName() = "GtmuxMobile"
  override fun createReactActivityDelegate(): ReactActivityDelegate = DefaultReactActivityDelegate(this, mainComponentName, fabricEnabled)
  private fun selection(active: Boolean) {
    val context = (application as ReactApplication).reactHost?.currentReactContext ?: return
    if (context.hasActiveReactInstance()) context.getJSModule(DeviceEventManagerModule.RCTDeviceEventEmitter::class.java).emit("gtmuxTextSelection", active)
  }
  override fun onActionModeStarted(mode: ActionMode) { super.onActionModeStarted(mode); selection(true) }
  override fun onActionModeFinished(mode: ActionMode) { super.onActionModeFinished(mode); selection(false) }
  override fun dispatchKeyEvent(event: KeyEvent): Boolean {
    if (KeyCommandsModule.dispatch(this, event)) return true
    return super.dispatchKeyEvent(event)
  }
}
