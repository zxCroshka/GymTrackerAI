import type { Metadata } from "next";

import { ProfileView } from "@/features/profile/profile-view";

export const metadata: Metadata = { title: "Профиль" };

export default function ProfilePage() {
  return <ProfileView />;
}
