"use client";

import { createContext, useContext } from "react";
import type { User } from "@/lib/api";

// Profil user login di-fetch sekali oleh layout (app), lalu dibagikan ke
// seluruh halaman lewat context — hindari getMe() berulang per halaman.
// Bernilai null selama profil masih dimuat atau saat fetch gagal.
const CurrentUserContext = createContext<User | null>(null);

export function CurrentUserProvider({
  me,
  children,
}: {
  me: User | null;
  children: React.ReactNode;
}) {
  return (
    <CurrentUserContext.Provider value={me}>
      {children}
    </CurrentUserContext.Provider>
  );
}

export function useCurrentUser(): User | null {
  return useContext(CurrentUserContext);
}
