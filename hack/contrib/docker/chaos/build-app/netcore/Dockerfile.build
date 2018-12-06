FROM microsoft/dotnet:${DOTNET_SDK_VERSION:2.2-sdk-alpine}
WORKDIR /app

# copy csproj and restore as distinct layers
COPY . .
RUN ${DOTNET_RESTORE_PRE} && ${DOTNET_RESTORE:dotnet restore} && dotnet publish -c Release -o /out
CMD ["cp","-r","/out/","/tmp/"]