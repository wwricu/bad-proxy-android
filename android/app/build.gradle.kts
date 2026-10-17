plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    namespace = "icu.wwr.badproxy"
    compileSdk = 36

    defaultConfig {
        applicationId = "icu.wwr.badproxy"
        minSdk = 24
        targetSdk = 33 // pre-34: no foreground-service-type declaration needed
        versionCode = 1
        versionName = "0.0.1-beta-1"
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.getByName("debug")
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_11
        targetCompatibility = JavaVersion.VERSION_11
    }
    kotlinOptions {
        jvmTarget = "11"
    }
}

dependencies {
    implementation(files("libs/vpn.aar"))
}
