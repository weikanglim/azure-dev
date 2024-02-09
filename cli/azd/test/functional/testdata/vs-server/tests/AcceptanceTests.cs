using Should;
using System.Text.RegularExpressions;

namespace AzdVsServerTests;

[Parallelizable(ParallelScope.Self)]
[TestFixture]
public class AcceptanceTests : TestBase
{
    [Test]
    public async Task ManageEnvironments()
    {
        IObserver<ProgressMessage> observer = new WriterObserver<ProgressMessage>();
        var session = await svrSvc.InitializeAsync(_rootDir, CancellationToken.None);
        var result = await asSvc.GetAspireHostAsync(session, "Production", observer, CancellationToken.None);
        var environments = (await esSvc.GetEnvironmentsAsync(session, observer, CancellationToken.None)).ToList();
        environments.ShouldBeEmpty();

        Environment e = new Environment("env1") {
            Properties = new Dictionary<string, string>() {
                { "ASPIRE_ENVIRONMENT", "Production" },
                { "Subscription", _subscriptionId },
                { "Location", _location}
            },
            Services = [
                new Service() {
                    Name = "apiservice",
                    IsExternal = false,
                },
                new Service() {
                    Name = "webfrontend",
                    IsExternal = true,
                }
            ],
        };

        await esSvc.CreateEnvironmentAsync(session, e, observer, CancellationToken.None);

        environments = (await esSvc.GetEnvironmentsAsync(session, observer, CancellationToken.None)).ToList();
        environments.ShouldNotBeEmpty();
        environments.Count.ShouldEqual(1);
        environments[0].Name.ShouldEqual(e.Name);
        environments[0].IsCurrent.ShouldBeTrue();

        Environment e2 = new Environment("env2") {
            Properties = new Dictionary<string, string>() {
                { "ASPIRE_ENVIRONMENT", "Production" },
                { "Subscription", _subscriptionId },
                { "Location", _location}
            },
            Services = e.Services,
        };

        await esSvc.CreateEnvironmentAsync(session, e2, observer, CancellationToken.None);

        environments = (await esSvc.GetEnvironmentsAsync(session, observer, CancellationToken.None)).ToList();
        environments.ShouldNotBeEmpty();
        environments.Count.ShouldEqual(2);

        var openEnv = await esSvc.OpenEnvironmentAsync(session, e.Name, observer, CancellationToken.None);
        openEnv.Name.ShouldEqual(e.Name);
        openEnv.IsCurrent.ShouldBeFalse();

        openEnv = await esSvc.OpenEnvironmentAsync(session, e2.Name, observer, CancellationToken.None);
        openEnv.Name.ShouldEqual(e2.Name);
        openEnv.IsCurrent.ShouldBeTrue();

        await esSvc.SetCurrentEnvironmentAsync(session, e.Name, observer, CancellationToken.None);
        openEnv = await esSvc.OpenEnvironmentAsync(session, e.Name, observer, CancellationToken.None);
        openEnv.Name.ShouldEqual(e.Name);
        openEnv.IsCurrent.ShouldBeTrue();

        Console.WriteLine("== Just a message ==");

    //     var result = await esSvc.LoadEnvironmentAsync(session, Settings.EnvironmentName, observer, CancellationToken.None);
    //     WriteEnvironment(result);
    // { 
    //     Console.WriteLine($"== Refreshing Environment: {Settings.EnvironmentName} ==");
    //     var result = await esSvc.RefreshEnvironmentAsync(session, Settings.EnvironmentName, observer, CancellationToken.None);
    //     WriteEnvironment(result);
    //     Console.WriteLine($"== Done Refreshing Environment: {Settings.EnvironmentName} ==");
    // }

    // {
    //     Console.WriteLine($"== Deploying Environment: {Settings.EnvironmentName} ==");
    //     var result = await esSvc.DeployAsync(session, Settings.EnvironmentName, observer, CancellationToken.None);
    //     WriteEnvironment(result);
    //     Console.WriteLine("== Done Deploying Environment ==");
    // }

    // {
    //     Console.WriteLine($"== Setting Current Environment: {Settings.EnvironmentName} ==");
    //     var result = await esSvc.SetCurrentEnvironmentAsync(session, Settings.EnvironmentName, observer, CancellationToken.None);
    //     Console.WriteLine($"Result: {result}");
    //     Console.WriteLine("== Done Setting Current Environment ==");
    // }
    }
}
